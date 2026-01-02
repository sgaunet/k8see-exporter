package main_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	main "github.com/sgaunet/k8see-exporter"
)

// TestReadYAMLConfigFile_ValidFile tests reading a valid configuration file.
func TestReadYAMLConfigFile_ValidFile(t *testing.T) {
	cfg, err := main.ReadYAMLConfigFile("testdata/valid-config.yml")
	require.NoError(t, err)

	// Verify all fields are populated correctly
	assert.Equal(t, "localhost", cfg.RedisHost)
	assert.Equal(t, "6379", cfg.RedisPort)
	assert.Equal(t, "k8sevents", cfg.RedisStream)
	assert.Equal(t, 5000, cfg.RedisStreamMaxLength)
	assert.Equal(t, "2112", cfg.MetricsPort)

	// Async processing configuration
	assert.Equal(t, 10000, cfg.EventBufferSize)
	assert.Equal(t, 1, cfg.EventWorkers)
	assert.Equal(t, 30, cfg.ShutdownTimeout)

	// Circuit breaker configuration
	assert.True(t, cfg.CircuitBreakerEnabled)
	assert.Equal(t, uint32(3), cfg.CircuitBreakerMaxRequests)
	assert.Equal(t, 60, cfg.CircuitBreakerInterval)
	assert.Equal(t, 30, cfg.CircuitBreakerTimeout)
	assert.Equal(t, 0.6, cfg.CircuitBreakerFailureRatio)

	// Exponential backoff configuration
	assert.Equal(t, 500, cfg.BackoffInitialInterval)
	assert.Equal(t, 2.0, cfg.BackoffMultiplier)
	assert.Equal(t, 30000, cfg.BackoffMaxInterval)
	assert.Equal(t, 60000, cfg.BackoffMaxElapsedTime)
}

// TestReadYAMLConfigFile_NonExistentFile tests reading a file that doesn't exist.
func TestReadYAMLConfigFile_NonExistentFile(t *testing.T) {
	cfg, err := main.ReadYAMLConfigFile("testdata/does-not-exist.yml")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
	assert.Empty(t, cfg.RedisHost)
}

// TestReadYAMLConfigFile_InvalidYAML tests reading a malformed YAML file.
func TestReadYAMLConfigFile_InvalidYAML(t *testing.T) {
	cfg, err := main.ReadYAMLConfigFile("testdata/invalid-config.yml")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML")
	assert.Empty(t, cfg.RedisHost)
}

// TestReadYAMLConfigFile_EmptyFile tests reading an empty configuration file.
func TestReadYAMLConfigFile_EmptyFile(t *testing.T) {
	cfg, err := main.ReadYAMLConfigFile("testdata/empty-config.yml")

	require.Error(t, err)
	// Should fail validation for missing required fields
	assert.Contains(t, err.Error(), "invalid configuration")
	assert.Empty(t, cfg.RedisHost)
}

// TestReadYAMLConfigFile_PartialConfig tests reading a config with missing required fields.
func TestReadYAMLConfigFile_PartialConfig(t *testing.T) {
	cfg, err := main.ReadYAMLConfigFile("testdata/partial-config.yml")

	require.Error(t, err)
	// Should fail validation for missing redis_port and redis_stream
	assert.Contains(t, err.Error(), "invalid configuration")
	// RedisHost should be populated since it was in the file
	assert.Equal(t, "localhost", cfg.RedisHost)
}

// TestYamlConfig_Validate tests the Validate method with various configurations.
func TestYamlConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      main.YamlConfig
		expectedErr error
	}{
		{
			name: "valid config - all required fields present",
			config: main.YamlConfig{
				RedisHost:   "localhost",
				RedisPort:   "6379",
				RedisStream: "k8sevents",
			},
			expectedErr: nil,
		},
		{
			name: "missing redis_host",
			config: main.YamlConfig{
				RedisPort:   "6379",
				RedisStream: "k8sevents",
			},
			expectedErr: main.ErrRedisHostRequired,
		},
		{
			name: "missing redis_port",
			config: main.YamlConfig{
				RedisHost:   "localhost",
				RedisStream: "k8sevents",
			},
			expectedErr: main.ErrRedisPortRequired,
		},
		{
			name: "missing redis_stream",
			config: main.YamlConfig{
				RedisHost: "localhost",
				RedisPort: "6379",
			},
			expectedErr: main.ErrRedisStreamRequired,
		},
		{
			name:        "all required fields missing",
			config:      main.YamlConfig{},
			expectedErr: main.ErrRedisHostRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr),
					"expected error %v, got %v", tt.expectedErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestYamlConfig_SetDefaults_AllZeroValues tests that all defaults are applied
// when starting with a zero-value config.
func TestYamlConfig_SetDefaults_AllZeroValues(t *testing.T) {
	cfg := main.YamlConfig{}
	cfg.SetDefaults()

	// Async processing defaults
	assert.Equal(t, 10000, cfg.EventBufferSize, "EventBufferSize default")
	assert.Equal(t, 1, cfg.EventWorkers, "EventWorkers default")
	assert.Equal(t, 30, cfg.ShutdownTimeout, "ShutdownTimeout default")

	// Circuit breaker defaults
	// Note: CircuitBreakerEnabled is NOT defaulted (see config.go comment)
	assert.False(t, cfg.CircuitBreakerEnabled, "CircuitBreakerEnabled should remain false")
	assert.Equal(t, uint32(3), cfg.CircuitBreakerMaxRequests, "CircuitBreakerMaxRequests default")
	assert.Equal(t, 60, cfg.CircuitBreakerInterval, "CircuitBreakerInterval default")
	assert.Equal(t, 30, cfg.CircuitBreakerTimeout, "CircuitBreakerTimeout default")
	assert.Equal(t, 0.6, cfg.CircuitBreakerFailureRatio, "CircuitBreakerFailureRatio default")

	// Exponential backoff defaults
	assert.Equal(t, 500, cfg.BackoffInitialInterval, "BackoffInitialInterval default")
	assert.Equal(t, 2.0, cfg.BackoffMultiplier, "BackoffMultiplier default")
	assert.Equal(t, 30000, cfg.BackoffMaxInterval, "BackoffMaxInterval default")
	assert.Equal(t, 60000, cfg.BackoffMaxElapsedTime, "BackoffMaxElapsedTime default")
}

// TestYamlConfig_SetDefaults_PartialValues tests that only missing fields
// get defaults when some values are already set.
func TestYamlConfig_SetDefaults_PartialValues(t *testing.T) {
	cfg := main.YamlConfig{
		EventBufferSize:            5000,  // Custom value
		CircuitBreakerMaxRequests:  5,     // Custom value
		BackoffInitialInterval:     1000,  // Custom value
		// All other fields at zero value
	}

	cfg.SetDefaults()

	// Fields with custom values should NOT be changed
	assert.Equal(t, 5000, cfg.EventBufferSize, "Custom EventBufferSize preserved")
	assert.Equal(t, uint32(5), cfg.CircuitBreakerMaxRequests, "Custom CircuitBreakerMaxRequests preserved")
	assert.Equal(t, 1000, cfg.BackoffInitialInterval, "Custom BackoffInitialInterval preserved")

	// Fields with zero values should get defaults
	assert.Equal(t, 1, cfg.EventWorkers, "EventWorkers gets default")
	assert.Equal(t, 30, cfg.ShutdownTimeout, "ShutdownTimeout gets default")
	assert.Equal(t, 60, cfg.CircuitBreakerInterval, "CircuitBreakerInterval gets default")
	assert.Equal(t, 30, cfg.CircuitBreakerTimeout, "CircuitBreakerTimeout gets default")
	assert.Equal(t, 0.6, cfg.CircuitBreakerFailureRatio, "CircuitBreakerFailureRatio gets default")
	assert.Equal(t, 2.0, cfg.BackoffMultiplier, "BackoffMultiplier gets default")
	assert.Equal(t, 30000, cfg.BackoffMaxInterval, "BackoffMaxInterval gets default")
	assert.Equal(t, 60000, cfg.BackoffMaxElapsedTime, "BackoffMaxElapsedTime gets default")
}

// TestYamlConfig_SetDefaults_AllValuesSet tests that no changes occur
// when all values are already set to non-zero values.
func TestYamlConfig_SetDefaults_AllValuesSet(t *testing.T) {
	cfg := main.YamlConfig{
		EventBufferSize:            20000,
		EventWorkers:               5,
		ShutdownTimeout:            60,
		CircuitBreakerEnabled:      true,
		CircuitBreakerMaxRequests:  10,
		CircuitBreakerInterval:     120,
		CircuitBreakerTimeout:      60,
		CircuitBreakerFailureRatio: 0.8,
		BackoffInitialInterval:     1000,
		BackoffMultiplier:          1.5,
		BackoffMaxInterval:         60000,
		BackoffMaxElapsedTime:      120000,
	}

	original := cfg
	cfg.SetDefaults()

	// All fields should remain unchanged
	assert.Equal(t, original.EventBufferSize, cfg.EventBufferSize)
	assert.Equal(t, original.EventWorkers, cfg.EventWorkers)
	assert.Equal(t, original.ShutdownTimeout, cfg.ShutdownTimeout)
	assert.Equal(t, original.CircuitBreakerEnabled, cfg.CircuitBreakerEnabled)
	assert.Equal(t, original.CircuitBreakerMaxRequests, cfg.CircuitBreakerMaxRequests)
	assert.Equal(t, original.CircuitBreakerInterval, cfg.CircuitBreakerInterval)
	assert.Equal(t, original.CircuitBreakerTimeout, cfg.CircuitBreakerTimeout)
	assert.Equal(t, original.CircuitBreakerFailureRatio, cfg.CircuitBreakerFailureRatio)
	assert.Equal(t, original.BackoffInitialInterval, cfg.BackoffInitialInterval)
	assert.Equal(t, original.BackoffMultiplier, cfg.BackoffMultiplier)
	assert.Equal(t, original.BackoffMaxInterval, cfg.BackoffMaxInterval)
	assert.Equal(t, original.BackoffMaxElapsedTime, cfg.BackoffMaxElapsedTime)
}

// TestYamlConfig_SetDefaults_Idempotent tests that calling SetDefaults
// multiple times produces the same result.
func TestYamlConfig_SetDefaults_Idempotent(t *testing.T) {
	cfg := main.YamlConfig{
		EventBufferSize: 5000,
		// Other fields at zero value
	}

	// Call SetDefaults twice
	cfg.SetDefaults()
	afterFirst := cfg
	cfg.SetDefaults()
	afterSecond := cfg

	// Results should be identical
	assert.Equal(t, afterFirst, afterSecond, "SetDefaults should be idempotent")
}

// TestYamlConfig_SetDefaults_CircuitBreakerEnabledPreserved tests that
// CircuitBreakerEnabled=false is preserved (regression test for the bug
// where it was being defaulted to true).
func TestYamlConfig_SetDefaults_CircuitBreakerEnabledPreserved(t *testing.T) {
	tests := []struct {
		name          string
		initialValue  bool
		expectedValue bool
	}{
		{
			name:          "CircuitBreakerEnabled=false is preserved",
			initialValue:  false,
			expectedValue: false,
		},
		{
			name:          "CircuitBreakerEnabled=true is preserved",
			initialValue:  true,
			expectedValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := main.YamlConfig{
				CircuitBreakerEnabled: tt.initialValue,
			}

			cfg.SetDefaults()

			assert.Equal(t, tt.expectedValue, cfg.CircuitBreakerEnabled,
				"CircuitBreakerEnabled should preserve explicit value")
		})
	}
}

// TestYamlConfig_SetDefaults_FloatDefaults tests that float default values
// are set correctly (float64 fields).
func TestYamlConfig_SetDefaults_FloatDefaults(t *testing.T) {
	cfg := main.YamlConfig{}
	cfg.SetDefaults()

	// Test float64 fields specifically
	assert.Equal(t, 0.6, cfg.CircuitBreakerFailureRatio, "CircuitBreakerFailureRatio float default")
	assert.Equal(t, 2.0, cfg.BackoffMultiplier, "BackoffMultiplier float default")

	// Verify custom float values are preserved
	cfg2 := main.YamlConfig{
		CircuitBreakerFailureRatio: 0.75,
		BackoffMultiplier:          1.5,
	}
	cfg2.SetDefaults()

	assert.Equal(t, 0.75, cfg2.CircuitBreakerFailureRatio, "Custom float value preserved")
	assert.Equal(t, 1.5, cfg2.BackoffMultiplier, "Custom float value preserved")
}

// TestYamlConfig_SetDefaults_Uint32Default tests that uint32 default values
// are set correctly.
func TestYamlConfig_SetDefaults_Uint32Default(t *testing.T) {
	cfg := main.YamlConfig{}
	cfg.SetDefaults()

	// CircuitBreakerMaxRequests is uint32
	assert.Equal(t, uint32(3), cfg.CircuitBreakerMaxRequests, "uint32 default value")

	// Verify custom uint32 value is preserved
	cfg2 := main.YamlConfig{
		CircuitBreakerMaxRequests: 100,
	}
	cfg2.SetDefaults()

	assert.Equal(t, uint32(100), cfg2.CircuitBreakerMaxRequests, "Custom uint32 value preserved")
}
