package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
)

// Config represents the root configuration structure
type Config struct {
	Version string        `yaml:"version"`
	Server  ServerConfig  `yaml:"server"`
	Storage StorageConfig `yaml:"storage"`
	Jobs    []JobConfig   `yaml:"jobs"`
}

// ServerConfig contains settings for the HTTP server
type ServerConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

// StorageConfig contains settings for backup storage
type StorageConfig struct {
	Type  string      `yaml:"type"`
	Local LocalConfig `yaml:"local,omitempty"`
}

// LocalConfig contains settings for local file storage
type LocalConfig struct {
	Directory string `yaml:"directory"`
	MaxSize   string `yaml:"max_size"`
}

// JobConfig represents a single backup job configuration
type JobConfig struct {
	Name            string          `yaml:"name"`
	Description     string          `yaml:"description"`
	Type            string          `yaml:"type"`
	PostgresConfig  *PostgresConfig `yaml:"postgres_config,omitempty"`
	MySQLConfig     *MySQLConfig    `yaml:"mysql_config,omitempty"`
	MinIOConfig     *MinIOConfig    `yaml:"minio_config,omitempty"`
	Schedule        string          `yaml:"schedule"`
	RetentionPolicy RetentionPolicy `yaml:"retention_policy"`
	Notification    Notification    `yaml:"notification"`
}

// PostgresConfig contains PostgreSQL specific backup settings
type PostgresConfig struct {
	Host     string            `yaml:"host"`
	Port     string            `yaml:"port,omitempty"`
	User     string            `yaml:"user,omitempty"`
	Password string            `yaml:"password,omitempty"`
	Database string            `yaml:"database"`
	Options  map[string]string `yaml:"options,omitempty"` // Additional pg_dump options
}

// MySQLConfig contains MySQL specific backup settings
type MySQLConfig struct {
	ConnectionString string `yaml:"connection_string"`
}

// MinIOConfig contains MinIO specific backup settings
type MinIOConfig struct {
	Endpoint     string `yaml:"endpoint"`
	AccessKey    string `yaml:"access_key"`
	SecretKey    string `yaml:"secret_key"`
	BucketName   string `yaml:"bucket_name"`
	UseSSL       bool   `yaml:"use_ssl"`
	SourceFolder string `yaml:"source_folder"`
}

// RetentionPolicy defines how long backups are kept
type RetentionPolicy struct {
	Type  string `yaml:"type"` // "count" or "days"
	Value int    `yaml:"value"`
}

// Notification defines notification settings for backup jobs
type Notification struct {
	Enabled bool             `yaml:"enabled"`
	Discord *DiscordSettings `yaml:"discord,omitempty"`
	Webhook *WebhookSettings `yaml:"webhook,omitempty"`
}

// DiscordSettings contains Discord notification configuration
type DiscordSettings struct {
	When       []string `yaml:"when"`
	WebhookURL string   `yaml:"webhook_url"`
}

// WebhookSettings contains external webhook notification configuration
type WebhookSettings struct {
	URL         string            `yaml:"url"`
	Headers     map[string]string `yaml:"headers,omitempty"`
	AuthToken   string            `yaml:"auth_token,omitempty"`
	ContentType string            `yaml:"content_type,omitempty"`
}

// LoadConfig loads configuration from the specified YAML file
func LoadConfig(path string) (*Config, error) {
	// Expand home directory if path starts with ~
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to expand home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Process environment variables in the raw YAML content before unmarshaling
	processedData, unresolvedVars, err := replaceEnvVarsInYAML(string(data))
	if err != nil {
		return nil, err
	}

	// Report unresolved environment variables
	if len(unresolvedVars) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(unresolvedVars, ", "))
	}

	var config Config
	if err := yaml.Unmarshal([]byte(processedData), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// replaceEnvVarsInYAML replaces environment variable placeholders in the raw YAML content
// Returns the processed YAML content and a list of any unresolved environment variables
func replaceEnvVarsInYAML(yamlContent string) (string, []string, error) {
	// Regex to match string values potentially containing ${ENV_VAR} patterns
	// This looks for strings that might contain environment variables
	re := regexp.MustCompile(`:\s*"([^"]*\${[A-Za-z0-9_]+}[^"]*)"`)

	// Track unresolved environment variables
	var unresolvedVars []string

	processedContent := re.ReplaceAllStringFunc(yamlContent, func(match string) string {
		// Extract quoted value part
		parts := re.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}

		valueWithEnvVars := parts[1]
		processedValue, unresolved := replaceEnvVars(valueWithEnvVars)

		// Track unresolved variables
		unresolvedVars = append(unresolvedVars, unresolved...)

		// Reconstruct the YAML line with the processed value
		return strings.Replace(match, parts[1], processedValue, 1)
	})

	return processedContent, unresolvedVars, nil
}

// replaceEnvVars replaces ${ENV_VAR} patterns with environment variable values
// Returns the processed string and a list of unresolved environment variables
func replaceEnvVars(value string) (string, []string) {
	// Regex to match ${ENV_VAR} pattern
	re := regexp.MustCompile(`\${([A-Za-z0-9_]+)}`)

	var unresolvedVars []string

	result := re.ReplaceAllStringFunc(value, func(match string) string {
		// Extract the environment variable name (remove ${ and })
		envVar := match[2 : len(match)-1]

		// Get the environment variable value
		envValue := os.Getenv(envVar)

		// If the environment variable is not set, track it as unresolved
		if envValue == "" {
			// Check if it's an optional variable (marked with a '?' suffix)
			if !strings.HasPrefix(envVar, "?") {
				unresolvedVars = append(unresolvedVars, envVar)
			}
			return match
		}

		return envValue
	})

	return result, unresolvedVars
}

// MarkEnvVarOptional helps to document that a specific environment variable is optional in the configuration
// This is just a helper function to make code more expressive
func MarkEnvVarOptional(varName string) string {
	return fmt.Sprintf("${?%s}", varName)
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Check server configuration
	if c.Server.Enabled && (c.Server.Port <= 0 || c.Server.Port > 65535) {
		return fmt.Errorf("server port must be between 1 and 65535")
	}

	// Check storage configuration
	if c.Storage.Type == "local" {
		if c.Storage.Local.Directory == "" {
			return fmt.Errorf("local storage directory must be specified")
		}
	} else {
		return fmt.Errorf("unsupported storage type: %s", c.Storage.Type)
	}

	// Check jobs configuration
	if len(c.Jobs) == 0 {
		return fmt.Errorf("at least one job must be configured")
	}

	for i, job := range c.Jobs {
		if job.Name == "" {
			return fmt.Errorf("job #%d has no name", i+1)
		}

		// Check job type and required configuration
		switch job.Type {
		case "postgres":
			if job.PostgresConfig == nil {
				return fmt.Errorf("postgres job '%s' must have configuration", job.Name)
			}

			// Check required PostgreSQL parameters
			if job.PostgresConfig.Host == "" {
				return fmt.Errorf("postgres job '%s' must have a host", job.Name)
			}
			if job.PostgresConfig.Database == "" {
				return fmt.Errorf("postgres job '%s' must have a database name", job.Name)
			}
		case "mysql":
			if job.MySQLConfig == nil || job.MySQLConfig.ConnectionString == "" {
				return fmt.Errorf("mysql job '%s' must have a valid connection string", job.Name)
			}
		case "minio":
			if job.MinIOConfig == nil || job.MinIOConfig.Endpoint == "" ||
				job.MinIOConfig.BucketName == "" {
				return fmt.Errorf("minio job '%s' must have a valid endpoint and bucket name", job.Name)
			}
		default:
			return fmt.Errorf("unsupported job type '%s' for job '%s'", job.Type, job.Name)
		}

		// Check schedule
		if job.Schedule == "" {
			return fmt.Errorf("job '%s' has no schedule", job.Name)
		}

		// Check retention policy
		if job.RetentionPolicy.Type != "count" && job.RetentionPolicy.Type != "days" {
			return fmt.Errorf("job '%s' has invalid retention policy type: %s", job.Name, job.RetentionPolicy.Type)
		}
		if job.RetentionPolicy.Value <= 0 {
			return fmt.Errorf("job '%s' has invalid retention policy value: %d", job.Name, job.RetentionPolicy.Value)
		}
	}

	return nil
}
