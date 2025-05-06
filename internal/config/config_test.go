package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Test cases
	tests := []struct {
		name        string
		configData  string
		expectError bool
	}{
		{
			name: "valid config",
			configData: `
version: "1.0"
server:
  enabled: true
  port: 8080
storage:
  type: local
  local:
    directory: /path/to/storage
    max_size: 100GB
jobs:
  - name: "test job"
    description: "This is a test job"
    type: "postgres"
    postgres_config:
      host: "localhost"
      port: "5432"
      user: "postgres"
      password: "password"
      database: "dbname"
    schedule: "0 0 * * *"
    retention_policy:
      type: "count"
      value: 5
    notification:
      enabled: true
      discord:
        when:
          - "success"
          - "failure"
        webhook_url: "https://discord.com/api/webhooks/..."
`,
			expectError: false,
		},
		{
			name: "invalid yaml",
			configData: `
version: "1.0"
server:
  enabled: true
  port: 8080
storage:
  type: local
  local:
    directory: /path/to/storage
    max_size: 100GB
jobs:
  - name: "test job"
    type: "postgres"
    postgres_config:
      host: "localhost"
      database: "dbname"
    schedule: "0 0 * * *"
  retention_policy:  # Indentation error here
    type: "count"
    value: 5
`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary config file
			configPath := filepath.Join(tempDir, tt.name+"-config.yml")
			err := os.WriteFile(configPath, []byte(tt.configData), 0644)
			require.NoError(t, err)

			// Test loading the config
			cfg, err := LoadConfig(configPath)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: Config{
				Version: "1.0",
				Server: ServerConfig{
					Enabled: true,
					Port:    8080,
				},
				Storage: StorageConfig{
					Type: "local",
					Local: LocalConfig{
						Directory: "/path/to/storage",
						MaxSize:   "100GB",
					},
				},
				Jobs: []JobConfig{
					{
						Name:        "test job",
						Description: "This is a test job",
						Type:        "postgres",
						PostgresConfig: &PostgresConfig{
							Host:     "localhost",
							Port:     "5432",
							User:     "postgres",
							Password: "password",
							Database: "dbname",
						},
						Schedule: "0 0 * * *",
						RetentionPolicy: RetentionPolicy{
							Type:  "count",
							Value: 5,
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid server port",
			config: Config{
				Version: "1.0",
				Server: ServerConfig{
					Enabled: true,
					Port:    -1, // Invalid port
				},
				Storage: StorageConfig{
					Type: "local",
					Local: LocalConfig{
						Directory: "/path/to/storage",
						MaxSize:   "100GB",
					},
				},
				Jobs: []JobConfig{
					{
						Name:        "test job",
						Description: "This is a test job",
						Type:        "postgres",
						PostgresConfig: &PostgresConfig{
							Host:     "localhost",
							Database: "dbname",
						},
						Schedule: "0 0 * * *",
						RetentionPolicy: RetentionPolicy{
							Type:  "count",
							Value: 5,
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "server port must be between 1 and 65535",
		},
		{
			name: "missing local storage directory",
			config: Config{
				Version: "1.0",
				Server: ServerConfig{
					Enabled: true,
					Port:    8080,
				},
				Storage: StorageConfig{
					Type: "local",
					Local: LocalConfig{
						Directory: "", // Missing directory
						MaxSize:   "100GB",
					},
				},
				Jobs: []JobConfig{
					{
						Name:        "test job",
						Description: "This is a test job",
						Type:        "postgres",
						PostgresConfig: &PostgresConfig{
							Host:     "localhost",
							Database: "dbname",
						},
						Schedule: "0 0 * * *",
						RetentionPolicy: RetentionPolicy{
							Type:  "count",
							Value: 5,
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "local storage directory must be specified",
		},
		{
			name: "no jobs configured",
			config: Config{
				Version: "1.0",
				Server: ServerConfig{
					Enabled: true,
					Port:    8080,
				},
				Storage: StorageConfig{
					Type: "local",
					Local: LocalConfig{
						Directory: "/path/to/storage",
						MaxSize:   "100GB",
					},
				},
				Jobs: []JobConfig{}, // Empty jobs list
			},
			expectError: true,
			errorMsg:    "at least one job must be configured",
		},
		{
			name: "invalid job type",
			config: Config{
				Version: "1.0",
				Server: ServerConfig{
					Enabled: true,
					Port:    8080,
				},
				Storage: StorageConfig{
					Type: "local",
					Local: LocalConfig{
						Directory: "/path/to/storage",
						MaxSize:   "100GB",
					},
				},
				Jobs: []JobConfig{
					{
						Name:        "test job",
						Description: "This is a test job",
						Type:        "unknown", // Invalid job type
						Schedule:    "0 0 * * *",
						RetentionPolicy: RetentionPolicy{
							Type:  "count",
							Value: 5,
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "unsupported job type 'unknown'",
		},
		{
			name: "missing postgres config",
			config: Config{
				Version: "1.0",
				Server: ServerConfig{
					Enabled: true,
					Port:    8080,
				},
				Storage: StorageConfig{
					Type: "local",
					Local: LocalConfig{
						Directory: "/path/to/storage",
						MaxSize:   "100GB",
					},
				},
				Jobs: []JobConfig{
					{
						Name:        "test job",
						Description: "This is a test job",
						Type:        "postgres",
						// PostgresConfig missing
						Schedule: "0 0 * * *",
						RetentionPolicy: RetentionPolicy{
							Type:  "count",
							Value: 5,
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "postgres job 'test job' must have configuration",
		},
		{
			name: "missing postgres host",
			config: Config{
				Version: "1.0",
				Server: ServerConfig{
					Enabled: true,
					Port:    8080,
				},
				Storage: StorageConfig{
					Type: "local",
					Local: LocalConfig{
						Directory: "/path/to/storage",
						MaxSize:   "100GB",
					},
				},
				Jobs: []JobConfig{
					{
						Name:        "test job",
						Description: "This is a test job",
						Type:        "postgres",
						PostgresConfig: &PostgresConfig{
							Database: "dbname", // Missing host
						},
						Schedule: "0 0 * * *",
						RetentionPolicy: RetentionPolicy{
							Type:  "count",
							Value: 5,
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "postgres job 'test job' must have a host",
		},
		{
			name: "missing postgres database",
			config: Config{
				Version: "1.0",
				Server: ServerConfig{
					Enabled: true,
					Port:    8080,
				},
				Storage: StorageConfig{
					Type: "local",
					Local: LocalConfig{
						Directory: "/path/to/storage",
						MaxSize:   "100GB",
					},
				},
				Jobs: []JobConfig{
					{
						Name:        "test job",
						Description: "This is a test job",
						Type:        "postgres",
						PostgresConfig: &PostgresConfig{
							Host: "localhost", // Missing database
						},
						Schedule: "0 0 * * *",
						RetentionPolicy: RetentionPolicy{
							Type:  "count",
							Value: 5,
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "postgres job 'test job' must have a database name",
		},
		{
			name: "invalid retention policy",
			config: Config{
				Version: "1.0",
				Server: ServerConfig{
					Enabled: true,
					Port:    8080,
				},
				Storage: StorageConfig{
					Type: "local",
					Local: LocalConfig{
						Directory: "/path/to/storage",
						MaxSize:   "100GB",
					},
				},
				Jobs: []JobConfig{
					{
						Name:        "test job",
						Description: "This is a test job",
						Type:        "postgres",
						PostgresConfig: &PostgresConfig{
							Host:     "localhost",
							Database: "dbname",
						},
						Schedule: "0 0 * * *",
						RetentionPolicy: RetentionPolicy{
							Type:  "invalid", // Invalid retention type
							Value: 5,
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "job 'test job' has invalid retention policy type: invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnmarshal(t *testing.T) {
	// Create test YAML content
	yamlContent := `
version: "1.0"
server:
  enabled: true
  port: 8080
storage:
  type: local
  local:
    directory: /path/to/storage
    max_size: 100GB
jobs:
  - name: "postgres backup"
    description: "Backup PostgreSQL database"
    type: "postgres"
    postgres_config:
      host: "localhost"
      port: "5432"
      user: "user"
      password: "password"
      database: "dbname"
    schedule: "0 0 * * *"
    retention_policy:
      type: "count"
      value: 5
    notification:
      enabled: true
      discord:
        when:
          - "success"
          - "failure"
        webhook_url: "https://discord.com/api/webhooks/..."
  - name: "mysql backup"
    description: "Backup MySQL database"
    type: "mysql"
    mysql_config:
      connection_string: "mysql://user:password@localhost:3306/dbname"
    schedule: "0 12 * * *"
    retention_policy:
      type: "days"
      value: 7
    notification:
      enabled: false
  - name: "minio backup"
    description: "Backup MinIO bucket"
    type: "minio"
    minio_config:
      endpoint: "localhost:9000"
      access_key: "minio"
      secret_key: "minio123"
      bucket_name: "my-bucket"
      use_ssl: false
      source_folder: "documents"
    schedule: "0 3 * * *"
    retention_policy:
      type: "count"
      value: 3
    notification:
      enabled: true
      webhook:
        url: "https://example.com/webhook"
        auth_token: "secret"
        content_type: "application/json"
`

	// Parse the YAML
	var config Config
	err := yaml.Unmarshal([]byte(yamlContent), &config)
	require.NoError(t, err)

	// Verify the parsed values
	assert.Equal(t, "1.0", config.Version)

	// Server config
	assert.True(t, config.Server.Enabled)
	assert.Equal(t, 8080, config.Server.Port)

	// Storage config
	assert.Equal(t, "local", config.Storage.Type)
	assert.Equal(t, "/path/to/storage", config.Storage.Local.Directory)
	assert.Equal(t, "100GB", config.Storage.Local.MaxSize)

	// Jobs
	require.Len(t, config.Jobs, 3)

	// PostgreSQL job
	postgres := config.Jobs[0]
	assert.Equal(t, "postgres backup", postgres.Name)
	assert.Equal(t, "postgres", postgres.Type)
	assert.NotNil(t, postgres.PostgresConfig)
	assert.Equal(t, "localhost", postgres.PostgresConfig.Host)
	assert.Equal(t, "5432", postgres.PostgresConfig.Port)
	assert.Equal(t, "user", postgres.PostgresConfig.User)
	assert.Equal(t, "password", postgres.PostgresConfig.Password)
	assert.Equal(t, "dbname", postgres.PostgresConfig.Database)
	assert.Equal(t, "0 0 * * *", postgres.Schedule)
	assert.Equal(t, "count", postgres.RetentionPolicy.Type)
	assert.Equal(t, 5, postgres.RetentionPolicy.Value)
	assert.True(t, postgres.Notification.Enabled)
	assert.NotNil(t, postgres.Notification.Discord)
	assert.Contains(t, postgres.Notification.Discord.When, "success")
	assert.Contains(t, postgres.Notification.Discord.When, "failure")

	// MySQL job
	mysql := config.Jobs[1]
	assert.Equal(t, "mysql backup", mysql.Name)
	assert.Equal(t, "mysql", mysql.Type)
	assert.NotNil(t, mysql.MySQLConfig)
	assert.Equal(t, "mysql://user:password@localhost:3306/dbname", mysql.MySQLConfig.ConnectionString)
	assert.Equal(t, "0 12 * * *", mysql.Schedule)
	assert.Equal(t, "days", mysql.RetentionPolicy.Type)
	assert.Equal(t, 7, mysql.RetentionPolicy.Value)
	assert.False(t, mysql.Notification.Enabled)

	// MinIO job
	minio := config.Jobs[2]
	assert.Equal(t, "minio backup", minio.Name)
	assert.Equal(t, "minio", minio.Type)
	assert.NotNil(t, minio.MinIOConfig)
	assert.Equal(t, "localhost:9000", minio.MinIOConfig.Endpoint)
	assert.Equal(t, "minio", minio.MinIOConfig.AccessKey)
	assert.Equal(t, "minio123", minio.MinIOConfig.SecretKey)
	assert.Equal(t, "my-bucket", minio.MinIOConfig.BucketName)
	assert.False(t, minio.MinIOConfig.UseSSL)
	assert.Equal(t, "documents", minio.MinIOConfig.SourceFolder)
	assert.Equal(t, "0 3 * * *", minio.Schedule)
	assert.Equal(t, "count", minio.RetentionPolicy.Type)
	assert.Equal(t, 3, minio.RetentionPolicy.Value)
	assert.True(t, minio.Notification.Enabled)
	assert.NotNil(t, minio.Notification.Webhook)
	assert.Equal(t, "https://example.com/webhook", minio.Notification.Webhook.URL)
	assert.Equal(t, "secret", minio.Notification.Webhook.AuthToken)
	assert.Equal(t, "application/json", minio.Notification.Webhook.ContentType)
}

func TestEnvVarReplacement(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "config-env-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set up test environment variables
	os.Setenv("TEST_DB_PASSWORD", "secret123")
	os.Setenv("TEST_DB_HOST", "db.example.com")
	os.Setenv("TEST_ENDPOINT", "minio.example.com")
	os.Setenv("TEST_ACCESS_KEY", "EXAMPLEKEY")
	os.Setenv("TEST_MAX_SIZE", "500GB")
	defer func() {
		os.Unsetenv("TEST_DB_PASSWORD")
		os.Unsetenv("TEST_DB_HOST")
		os.Unsetenv("TEST_ENDPOINT")
		os.Unsetenv("TEST_ACCESS_KEY")
		os.Unsetenv("TEST_MAX_SIZE")
	}()

	// Test cases
	tests := []struct {
		name        string
		configData  string
		setupEnv    func()
		cleanupEnv  func()
		expectError bool
		validate    func(*testing.T, *Config)
	}{
		{
			name: "successful replacement",
			configData: `
version: "1.0"
server:
  enabled: true
  port: 8080
storage:
  type: local
  local:
    directory: "/path/to/storage"
    max_size: "${TEST_MAX_SIZE}"
jobs:
  - name: "test job"
    description: "This is a test job"
    type: "postgres"
    postgres_config:
      host: "${TEST_DB_HOST}"
      port: "5432"
      user: "postgres"
      password: "${TEST_DB_PASSWORD}"
      database: "dbname"
    schedule: "0 0 * * *"
    retention_policy:
      type: "count"
      value: 5
    notification:
      enabled: true
      discord:
        when:
          - "success"
          - "failure"
        webhook_url: "https://discord.com/api/webhooks/..."
  - name: "minio backup"
    description: "Backup MinIO bucket"
    type: "minio"
    minio_config:
      endpoint: "${TEST_ENDPOINT}"
      access_key: "${TEST_ACCESS_KEY}"
      secret_key: "minio123"
      bucket_name: "my-bucket"
      use_ssl: false
      source_folder: "documents"
    schedule: "0 3 * * *"
    retention_policy:
      type: "count"
      value: 3
    notification:
      enabled: false
`,
			expectError: false,
			validate: func(t *testing.T, cfg *Config) {
				// Check environment variables were replaced
				assert.Equal(t, "500GB", cfg.Storage.Local.MaxSize)
				assert.Equal(t, "db.example.com", cfg.Jobs[0].PostgresConfig.Host)
				assert.Equal(t, "secret123", cfg.Jobs[0].PostgresConfig.Password)
				assert.Equal(t, "minio.example.com", cfg.Jobs[1].MinIOConfig.Endpoint)
				assert.Equal(t, "EXAMPLEKEY", cfg.Jobs[1].MinIOConfig.AccessKey)
			},
		},
		{
			name: "missing required environment variable",
			configData: `
version: "1.0"
server:
  enabled: true
  port: 8080
storage:
  type: local
  local:
    directory: "/path/to/storage"
    max_size: "100GB"
jobs:
  - name: "test job"
    description: "This is a test job"
    type: "postgres"
    postgres_config:
      host: "localhost"
      port: "5432"
      user: "postgres"
      password: "${MISSING_PASSWORD}"
      database: "dbname"
    schedule: "0 0 * * *"
    retention_policy:
      type: "count"
      value: 5
    notification:
      enabled: false
`,
			expectError: true,
		},
		{
			name: "optional environment variable",
			configData: `
version: "1.0"
server:
  enabled: true
  port: 8080
storage:
  type: local
  local:
    directory: "/path/to/storage"
    max_size: "${?OPTIONAL_ENV_VAR}"
jobs:
  - name: "test job"
    description: "This is a test job"
    type: "postgres"
    postgres_config:
      host: "localhost"
      port: "5432"
      user: "postgres"
      password: "${TEST_DB_PASSWORD}"
      database: "dbname"
    schedule: "0 0 * * *"
    retention_policy:
      type: "count"
      value: 5
    notification:
      enabled: false
`,
			expectError: false,
			validate: func(t *testing.T, cfg *Config) {
				// Check required env was replaced, optional remains as is
				assert.Equal(t, "${?OPTIONAL_ENV_VAR}", cfg.Storage.Local.MaxSize)
				assert.Equal(t, "secret123", cfg.Jobs[0].PostgresConfig.Password)
			},
		},
		{
			name: "multiple env vars in single string",
			configData: `
version: "1.0"
server:
  enabled: true
  port: 8080
storage:
  type: local
  local:
    directory: "/path/to/${TEST_ACCESS_KEY}/storage"
    max_size: "100GB"
jobs:
  - name: "test job"
    description: "This is a test job"
    type: "postgres"
    postgres_config:
      host: "${TEST_DB_HOST}"
      port: "5432"
      user: "postgres"
      password: "${TEST_DB_PASSWORD}"
      database: "dbname"
      options:
        schema-only: ""
        exclude-schema: "logs_${TEST_ACCESS_KEY}"
    schedule: "0 0 * * *"
    retention_policy:
      type: "count"
      value: 5
    notification:
      enabled: false
`,
			expectError: false,
			validate: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "/path/to/EXAMPLEKEY/storage", cfg.Storage.Local.Directory)
				assert.Equal(t, "db.example.com", cfg.Jobs[0].PostgresConfig.Host)
				assert.Equal(t, "secret123", cfg.Jobs[0].PostgresConfig.Password)
				assert.Equal(t, "logs_EXAMPLEKEY", cfg.Jobs[0].PostgresConfig.Options["exclude-schema"])
			},
		},
		{
			name: "set and unset env var during test",
			configData: `
version: "1.0"
server:
  enabled: true
  port: 8080
storage:
  type: local
  local:
    directory: "/path/to/storage"
    max_size: "${DYNAMIC_ENV_VAR}"
jobs:
  - name: "test job"
    description: "This is a test job"
    type: "postgres"
    postgres_config:
      host: "localhost"
      user: "postgres"
      password: "password"
      database: "dbname"
    schedule: "0 0 * * *"
    retention_policy:
      type: "count"
      value: 5
    notification:
      enabled: false
`,
			setupEnv: func() {
				os.Setenv("DYNAMIC_ENV_VAR", "200GB")
			},
			cleanupEnv: func() {
				os.Unsetenv("DYNAMIC_ENV_VAR")
			},
			expectError: false,
			validate: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "200GB", cfg.Storage.Local.MaxSize)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up dynamic environment if needed
			if tt.setupEnv != nil {
				tt.setupEnv()
			}

			// Clean up dynamic environment after test
			if tt.cleanupEnv != nil {
				defer tt.cleanupEnv()
			}

			// Create a temporary config file
			configPath := filepath.Join(tempDir, tt.name+"-config.yml")
			err := os.WriteFile(configPath, []byte(tt.configData), 0644)
			require.NoError(t, err)

			// Test loading the config
			cfg, err := LoadConfig(configPath)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)

				// Run validation logic if provided
				if tt.validate != nil {
					tt.validate(t, cfg)
				}
			}
		})
	}
}

func TestReplaceEnvVars(t *testing.T) {
	// Set up test environment variables
	os.Setenv("TEST_VAR1", "value1")
	os.Setenv("TEST_VAR2", "value2")
	defer func() {
		os.Unsetenv("TEST_VAR1")
		os.Unsetenv("TEST_VAR2")
	}()

	tests := []struct {
		name       string
		input      string
		expected   string
		unresolved int
	}{
		{
			name:       "single env var",
			input:      "prefix_${TEST_VAR1}_suffix",
			expected:   "prefix_value1_suffix",
			unresolved: 0,
		},
		{
			name:       "multiple env vars",
			input:      "${TEST_VAR1} and ${TEST_VAR2}",
			expected:   "value1 and value2",
			unresolved: 0,
		},
		{
			name:       "missing env var",
			input:      "${MISSING_VAR}",
			expected:   "${MISSING_VAR}",
			unresolved: 1,
		},
		{
			name:       "mixed existing and missing",
			input:      "${TEST_VAR1} and ${MISSING_VAR}",
			expected:   "value1 and ${MISSING_VAR}",
			unresolved: 1,
		},
		{
			name:       "no env vars",
			input:      "plain string",
			expected:   "plain string",
			unresolved: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, unresolved := replaceEnvVars(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.unresolved, len(unresolved))
		})
	}
}

func TestReplaceEnvVarsInYAML(t *testing.T) {
	// Set up test environment variables
	os.Setenv("TEST_PASSWORD", "secret")
	os.Setenv("TEST_HOST", "example.com")
	defer func() {
		os.Unsetenv("TEST_PASSWORD")
		os.Unsetenv("TEST_HOST")
	}()

	yamlContent := `
server:
  host: "${TEST_HOST}"
  port: 8080
database:
  connection_string: "postgresql://user:${TEST_PASSWORD}@localhost:5432/dbname"
  max_connections: 100
`

	expected := `
server:
  host: "example.com"
  port: 8080
database:
  connection_string: "postgresql://user:secret@localhost:5432/dbname"
  max_connections: 100
`

	processed, unresolved, err := replaceEnvVarsInYAML(yamlContent)
	assert.NoError(t, err)
	assert.Empty(t, unresolved)
	assert.Equal(t, expected, processed)
}

func TestMarkEnvVarOptional(t *testing.T) {
	result := MarkEnvVarOptional("TEST_VAR")
	assert.Equal(t, "${?TEST_VAR}", result)
}
