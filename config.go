// Package main contains the k8see-exporter application for exporting Kubernetes events to Redis.
package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// YamlConfig struct representing the yaml configuration file passed as a parameter to the program.
type YamlConfig struct {
	RedisHost            string `yaml:"redis_host"`
	RedisPort            string `yaml:"redis_port"`
	RedisPassword        string `yaml:"redis_password"`
	RedisStream          string `yaml:"redis_stream"`
	RedisStreamMaxLength int    `yaml:"redis_stream_maxlength"`
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

	return yamlConfig, nil
}
