package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thitiph0n/backmeup/internal/config"
	"github.com/thitiph0n/backmeup/internal/storage/localfs"
)

type fakeObjectStore struct {
	objects  []minio.ObjectInfo
	failKey  string
	blockGet bool
	gotPrefx string
}

func (f *fakeObjectStore) ListObjects(ctx context.Context, _ string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	f.gotPrefx = opts.Prefix
	ch := make(chan minio.ObjectInfo)
	go func() {
		defer close(ch)
		for _, o := range f.objects {
			select {
			case ch <- o:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

func (f *fakeObjectStore) FGetObject(ctx context.Context, _, objectName, filePath string, _ minio.GetObjectOptions) error {
	if f.blockGet {
		<-ctx.Done()
		return ctx.Err()
	}
	if objectName == f.failKey {
		return errors.New("boom")
	}
	return os.WriteFile(filePath, []byte("content of "+objectName), 0644)
}

func newMinioExecutor(t *testing.T, store *fakeObjectStore, sourceFolder string) (*MinioExecutor, string) {
	t.Helper()
	dir := t.TempDir()
	return &MinioExecutor{
		BaseExecutor: BaseExecutor{
			Config: config.JobConfig{
				Name:        "job",
				MinIOConfig: &config.MinIOConfig{BucketName: "bucket", SourceFolder: sourceFolder},
			},
			Storage: localfs.New(config.LocalConfig{Directory: dir}),
		},
		client: store,
	}, filepath.Join(dir, "job")
}

func singleBackupDir(t *testing.T, jobDir string) string {
	t.Helper()
	entries, err := os.ReadDir(jobDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	return filepath.Join(jobDir, entries[0].Name())
}

func TestMinioExecute_DownloadsObjectsKeepingRelativePaths(t *testing.T) {
	modTime := time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeObjectStore{objects: []minio.ObjectInfo{
		{Key: "data/a.txt", LastModified: modTime},
		{Key: "data/nested/deep/b.txt", LastModified: modTime},
		{Key: "data/marker/"},
	}}
	exec, jobDir := newMinioExecutor(t, store, "/data")

	require.NoError(t, exec.Execute(context.Background()))

	assert.Equal(t, "data/", store.gotPrefx)
	backupDir := singleBackupDir(t, jobDir)

	content, err := os.ReadFile(filepath.Join(backupDir, "nested", "deep", "b.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content of data/nested/deep/b.txt", string(content))

	info, err := os.Stat(filepath.Join(backupDir, "a.txt"))
	require.NoError(t, err)
	assert.True(t, info.ModTime().Equal(modTime))

	_, err = os.Stat(filepath.Join(backupDir, "marker"))
	assert.True(t, os.IsNotExist(err))
}

func TestMinioExecute_EmptyBucketSucceeds(t *testing.T) {
	exec, jobDir := newMinioExecutor(t, &fakeObjectStore{}, "")

	require.NoError(t, exec.Execute(context.Background()))

	assert.DirExists(t, singleBackupDir(t, jobDir))
}

func TestMinioExecute_TraversalKeyFailsAndCleansUp(t *testing.T) {
	store := &fakeObjectStore{objects: []minio.ObjectInfo{{Key: "../../escape.txt"}}}
	exec, jobDir := newMinioExecutor(t, store, "")

	err := exec.Execute(context.Background())

	require.ErrorContains(t, err, "escapes the backup directory")
	entries, readErr := os.ReadDir(jobDir)
	require.NoError(t, readErr)
	assert.Empty(t, entries)
	_, statErr := os.Stat(filepath.Join(filepath.Dir(jobDir), "escape.txt"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestMinioExecute_DownloadFailureRemovesBackupDir(t *testing.T) {
	store := &fakeObjectStore{
		objects: []minio.ObjectInfo{{Key: "ok.txt"}, {Key: "bad.txt"}},
		failKey: "bad.txt",
	}
	exec, jobDir := newMinioExecutor(t, store, "")

	err := exec.Execute(context.Background())

	require.ErrorContains(t, err, "bad.txt")
	entries, readErr := os.ReadDir(jobDir)
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

func TestMinioExecute_ContextCancellation(t *testing.T) {
	store := &fakeObjectStore{objects: []minio.ObjectInfo{{Key: "a.txt"}}, blockGet: true}
	exec, jobDir := newMinioExecutor(t, store, "")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := exec.Execute(ctx)

	require.Error(t, err)
	entries, readErr := os.ReadDir(jobDir)
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

func TestObjectDestination(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "backups", "job", "dir")
	tests := []struct {
		name    string
		prefix  string
		key     string
		want    string
		wantErr bool
	}{
		{"plain", "", "a/b.txt", filepath.Join(root, "a", "b.txt"), false},
		{"prefix stripped", "data/", "data/a.txt", filepath.Join(root, "a.txt"), false},
		{"parent traversal", "", "../x", "", true},
		{"nested traversal", "data/", "data/../../x", "", true},
		{"absolute key", "", "/etc/passwd", "", true},
		{"prefix only", "data/", "data/", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := objectDestination(root, tt.prefix, tt.key)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSourcePrefix(t *testing.T) {
	assert.Equal(t, "", sourcePrefix(""))
	assert.Equal(t, "", sourcePrefix("/"))
	assert.Equal(t, "data/", sourcePrefix("data"))
	assert.Equal(t, "data/sub/", sourcePrefix("/data/sub/"))
}
