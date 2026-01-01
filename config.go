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
