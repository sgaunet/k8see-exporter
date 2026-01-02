// Package main contains the k8see-exporter application for exporting Kubernetes events to Redis.
package main

import (
	"fmt"
	"os"

	"errors"

	"gopkg.in/yaml.v2"
)

var (
	// ErrRedisHostRequired is returned when redis_host is not configured.
	ErrRedisHostRequired = errors.New("redis_host is required")
	// ErrRedisPortRequired is returned when redis_port is not configured.
	ErrRedisPortRequired = errors.New("redis_port is required")
	// ErrRedisStreamRequired is returned when redis_stream is not configured.
	ErrRedisStreamRequired = errors.New("redis_stream is required")
)

// YamlConfig struct representing the yaml configuration file passed as a parameter to the program.
type YamlConfig struct {
	RedisHost            string `yaml:"redis_host"`
	RedisPort            string `yaml:"redis_port"`
	RedisPassword        string `yaml:"redis_password"`
	RedisStream          string `yaml:"redis_stream"`
	RedisStreamMaxLength int    `yaml:"redis_stream_maxlength"`
	MetricsPort          string `yaml:"metrics_port"`

	// Async processing configuration.
	EventBufferSize int `yaml:"event_buffer_size"` // Default: 10000
	EventWorkers    int `yaml:"event_workers"`     // Default: 1 (maintains ordering)
	ShutdownTimeout int `yaml:"shutdown_timeout_sec"` // Default: 30

	// Circuit breaker configuration.
	CircuitBreakerEnabled      bool    `yaml:"circuit_breaker_enabled"`       // Default: true
	CircuitBreakerMaxRequests  uint32  `yaml:"circuit_breaker_max_requests"`  // Default: 3
	CircuitBreakerInterval     int     `yaml:"circuit_breaker_interval_sec"`  // Default: 60
	CircuitBreakerTimeout      int     `yaml:"circuit_breaker_timeout_sec"`   // Default: 30
	CircuitBreakerFailureRatio float64 `yaml:"circuit_breaker_failure_ratio"` // Default: 0.6

	// Exponential backoff configuration.
	BackoffInitialInterval int     `yaml:"backoff_initial_interval_ms"` // Default: 500
	BackoffMultiplier      float64 `yaml:"backoff_multiplier"`          // Default: 2.0
	BackoffMaxInterval     int     `yaml:"backoff_max_interval_ms"`     // Default: 30000
	BackoffMaxElapsedTime  int     `yaml:"backoff_max_elapsed_time_ms"` // Default: 60000
}

// ReadYAMLConfigFile reads and parses a YAML configuration file.
func ReadYAMLConfigFile(filename string) (YamlConfig, error) {
	var yamlConfig YamlConfig

	yamlFile, err := os.ReadFile(filename) // #nosec G304 - filename comes from CLI flag
	if err != nil {
		fmt.Printf("Error reading YAML file: %s\n", err)
		return yamlConfig, fmt.Errorf("failed to read config file: %w", err)
	}

	err = yaml.Unmarshal(yamlFile, &yamlConfig)
	if err != nil {
		fmt.Printf("Error parsing YAML file: %s\n", err)
		return yamlConfig, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if err := yamlConfig.Validate(); err != nil {
		return yamlConfig, fmt.Errorf("invalid configuration: %w", err)
	}

	return yamlConfig, nil
}

// SetDefaults sets default values for optional configuration fields.
//
//nolint:cyclop // Complexity acceptable for configuration defaults
func (c *YamlConfig) SetDefaults() {
	// Async processing defaults
	if c.EventBufferSize == 0 {
		c.EventBufferSize = 10000
	}
	if c.EventWorkers == 0 {
		c.EventWorkers = 1 // Single worker maintains ordering
	}
	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = 30
	}

	// Circuit breaker defaults (enabled by default)
	if !c.CircuitBreakerEnabled {
		c.CircuitBreakerEnabled = true
	}
	if c.CircuitBreakerMaxRequests == 0 {
		c.CircuitBreakerMaxRequests = 3
	}
	if c.CircuitBreakerInterval == 0 {
		c.CircuitBreakerInterval = 60
	}
	if c.CircuitBreakerTimeout == 0 {
		c.CircuitBreakerTimeout = 30
	}
	if c.CircuitBreakerFailureRatio == 0 {
		c.CircuitBreakerFailureRatio = 0.6
	}

	// Exponential backoff defaults
	if c.BackoffInitialInterval == 0 {
		c.BackoffInitialInterval = 500
	}
	if c.BackoffMultiplier == 0 {
		c.BackoffMultiplier = 2.0
	}
	if c.BackoffMaxInterval == 0 {
		c.BackoffMaxInterval = 30000
	}
	if c.BackoffMaxElapsedTime == 0 {
		c.BackoffMaxElapsedTime = 60000
	}
}

// Validate checks that required configuration fields are set.
func (c *YamlConfig) Validate() error {
	if c.RedisHost == "" {
		return ErrRedisHostRequired
	}
	if c.RedisPort == "" {
		return ErrRedisPortRequired
	}
	if c.RedisStream == "" {
		return ErrRedisStreamRequired
	}
	return nil
}
