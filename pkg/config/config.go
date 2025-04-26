package config

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	Port string `yaml:"port"`
}

type ProxyConfig struct {
	Path   string `yaml:"path"`
	Target string `yaml:"target"`
	// AuthHeader string `yaml:"auth_header"`
	// ApiKey     string `yaml:"api_key"`
}

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Proxies []ProxyConfig `yaml:"proxies"`
}

func LoadConfig(path string) (*Config, error) {
	// 尝试读取配置文件
	file, err := os.ReadFile(path)
	if err != nil {
		// 如果文件读取失败，检查是否是“文件不存在”错误
		if os.IsNotExist(err) {
			// 文件不存在，尝试从环境变量加载
			proxiesJSON := os.Getenv("PROXIES_CONFIG")
			if proxiesJSON != "" {
				var proxies []ProxyConfig
				// 从环境变量中反序列化 JSON
				if err := json.Unmarshal([]byte(proxiesJSON), &proxies); err != nil {
					return nil, fmt.Errorf("环境变量 PROXIES_CONFIG 解析失败: %w", err)
				}
				// 使用来自环境变量的代理配置创建 Config 结构体，服务器配置将使用零值
				cfg := &Config{
					Proxies: proxies,
				}
				return cfg, nil
			} else {
				// 文件未找到且环境变量未设置
				return nil, fmt.Errorf("config file not found at %s and PROXIES_CONFIG environment variable is not set: %w", path, err)
			}
		}
		// 如果是其他类型的错误，返回该错误
		return nil, fmt.Errorf("failed to read config file at %s: %w", path, err)
	}

	// 文件存在，反序列化 YAML
	var cfg Config
	if err := yaml.Unmarshal(file, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file at %s: %w", path, err)
	}

	return &cfg, nil
}
