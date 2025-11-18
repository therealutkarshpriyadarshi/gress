package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, CurrentConfigVersion, cfg.Version)
	assert.Equal(t, "gress-app", cfg.Application.Name)
	assert.Equal(t, "development", cfg.Application.Environment)
	assert.Equal(t, 10000, cfg.Engine.BufferSize)
	assert.Equal(t, 100, cfg.Engine.MaxConcurrency)
	assert.True(t, cfg.Metrics.Enabled)
}

func TestProductionConfig(t *testing.T) {
	cfg := ProductionConfig()

	assert.Equal(t, "production", cfg.Application.Environment)
	assert.Equal(t, 50000, cfg.Engine.BufferSize)
	assert.Equal(t, 500, cfg.Engine.MaxConcurrency)
	assert.Equal(t, "warn", cfg.Logging.Level)
}

func TestDevelopmentConfig(t *testing.T) {
	cfg := DevelopmentConfig()

	assert.Equal(t, "development", cfg.Application.Environment)
	assert.Equal(t, "fail-fast", cfg.ErrorHandling.Strategy)
	assert.False(t, cfg.ErrorHandling.EnableRetry)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

func TestLoadYAMLConfig(t *testing.T) {
	// Create temporary YAML config
	yamlContent := `
version: v1
application:
  name: test-app
  environment: test
engine:
  buffer_size: 5000
  max_concurrency: 50
logging:
  level: info
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.Write([]byte(yamlContent))
	require.NoError(t, err)
	tmpFile.Close()

	// Load config
	cfg, err := LoadConfig(tmpFile.Name())
	require.NoError(t, err)

	assert.Equal(t, "v1", cfg.Version)
	assert.Equal(t, "test-app", cfg.Application.Name)
	assert.Equal(t, "test", cfg.Application.Environment)
	assert.Equal(t, 5000, cfg.Engine.BufferSize)
	assert.Equal(t, 50, cfg.Engine.MaxConcurrency)
	assert.Equal(t, "info", cfg.Logging.Level)
}

func TestLoadJSONConfig(t *testing.T) {
	// Create temporary JSON config
	jsonContent := `{
  "version": "v1",
  "application": {
    "name": "test-app",
    "environment": "test"
  },
  "engine": {
    "buffer_size": 5000,
    "max_concurrency": 50
  },
  "logging": {
    "level": "info"
  }
}`
	tmpFile, err := os.CreateTemp("", "config-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.Write([]byte(jsonContent))
	require.NoError(t, err)
	tmpFile.Close()

	// Load config
	cfg, err := LoadConfig(tmpFile.Name())
	require.NoError(t, err)

	assert.Equal(t, "v1", cfg.Version)
	assert.Equal(t, "test-app", cfg.Application.Name)
	assert.Equal(t, 5000, cfg.Engine.BufferSize)
}

func TestLoadConfigWithDefaults(t *testing.T) {
	// Create minimal config
	yamlContent := `
version: v1
application:
  name: minimal-app
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.Write([]byte(yamlContent))
	require.NoError(t, err)
	tmpFile.Close()

	// Load config with defaults
	cfg, err := LoadConfigWithDefaults(tmpFile.Name())
	require.NoError(t, err)

	// Check that defaults were applied
	assert.Equal(t, "minimal-app", cfg.Application.Name)
	assert.Equal(t, 10000, cfg.Engine.BufferSize) // Default
	assert.Equal(t, 100, cfg.Engine.MaxConcurrency) // Default
	assert.Equal(t, "info", cfg.Logging.Level) // Default
}

func TestApplyEnvOverrides(t *testing.T) {
	cfg := DefaultConfig()

	// Set environment variables
	os.Setenv("GRESS_APPLICATION_NAME", "env-app")
	os.Setenv("GRESS_ENGINE_BUFFER_SIZE", "20000")
	os.Setenv("GRESS_LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("GRESS_APPLICATION_NAME")
		os.Unsetenv("GRESS_ENGINE_BUFFER_SIZE")
		os.Unsetenv("GRESS_LOG_LEVEL")
	}()

	err := ApplyEnvOverrides(cfg)
	require.NoError(t, err)

	assert.Equal(t, "env-app", cfg.Application.Name)
	assert.Equal(t, 20000, cfg.Engine.BufferSize)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "valid default config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "missing version",
			config: &Config{
				Application: ApplicationConfig{Name: "test"},
			},
			wantErr: true,
		},
		{
			name: "invalid buffer size",
			config: &Config{
				Version:     CurrentConfigVersion,
				Application: ApplicationConfig{Name: "test", Environment: "development"},
				Engine: EngineConfig{
					BufferSize:         -1,
					MaxConcurrency:     100,
					CheckpointInterval: 30 * time.Second,
					WatermarkInterval:  5 * time.Second,
					MetricsInterval:    10 * time.Second,
					CheckpointDir:      "./checkpoints",
				},
				ErrorHandling: ErrorHandlingConfig{Strategy: "retry-then-dlq"},
				State:         StateConfig{Backend: "memory"},
				Metrics:       MetricsConfig{Namespace: "gress"},
				Logging:       LoggingConfig{Level: "info", Format: "json", Output: "stdout"},
				Security:      SecurityConfig{AuthType: "none"},
			},
			wantErr: true,
		},
		{
			name: "invalid environment",
			config: &Config{
				Version: CurrentConfigVersion,
				Application: ApplicationConfig{
					Name:        "test",
					Environment: "invalid-env",
				},
				Engine: EngineConfig{
					BufferSize:         10000,
					MaxConcurrency:     100,
					CheckpointInterval: 30 * time.Second,
					WatermarkInterval:  5 * time.Second,
					MetricsInterval:    10 * time.Second,
					CheckpointDir:      "./checkpoints",
				},
				ErrorHandling: ErrorHandlingConfig{Strategy: "retry-then-dlq"},
				State:         StateConfig{Backend: "memory"},
				Metrics:       MetricsConfig{Namespace: "gress"},
				Logging:       LoggingConfig{Level: "info", Format: "json", Output: "stdout"},
				Security:      SecurityConfig{AuthType: "none"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSaveConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Application.Name = "save-test"

	// Save as YAML
	yamlFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(yamlFile.Name())
	yamlFile.Close()

	err = SaveConfig(cfg, yamlFile.Name())
	require.NoError(t, err)

	// Load and verify
	loaded, err := LoadConfig(yamlFile.Name())
	require.NoError(t, err)
	assert.Equal(t, "save-test", loaded.Application.Name)

	// Save as JSON
	jsonFile, err := os.CreateTemp("", "config-*.json")
	require.NoError(t, err)
	defer os.Remove(jsonFile.Name())
	jsonFile.Close()

	err = SaveConfig(cfg, jsonFile.Name())
	require.NoError(t, err)

	// Load and verify
	loaded, err = LoadConfig(jsonFile.Name())
	require.NoError(t, err)
	assert.Equal(t, "save-test", loaded.Application.Name)
}

func TestMergeConfigs(t *testing.T) {
	base := DefaultConfig()
	base.Application.Name = "base"
	base.Engine.BufferSize = 1000

	override := &Config{
		Application: ApplicationConfig{Name: "override"},
		Engine:      EngineConfig{MaxConcurrency: 200},
	}

	merged := MergeConfigs(base, override)

	assert.Equal(t, "override", merged.Application.Name)
	assert.Equal(t, 1000, merged.Engine.BufferSize) // From base
	assert.Equal(t, 200, merged.Engine.MaxConcurrency) // From override
}
