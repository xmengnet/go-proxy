package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Port string `yaml:"port"`
}

type ProxyConfig struct {
	Path       string `yaml:"path"`
	Target     string `yaml:"target"`
	AuthHeader string `yaml:"auth_header"`
	// ApiKey     string `yaml:"api_key"`
}

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Proxies []ProxyConfig `yaml:"proxies"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(file, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
