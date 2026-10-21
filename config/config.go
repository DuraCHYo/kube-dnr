package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	Kubernetes KubernetesConfig `json:"kubernetes"`
	S3         S3Config         `json:"s3"`
}

type KubernetesConfig struct {
	ExcludedResources  []string `json:"excludedresources"`
	ExcludedNamespaces []string `json:"excludednamespaces"`
	ExcludedSubstrings []string `json:"excludedsubstrings"`
}

type S3Config struct {
	Bucket          string `json:"bucket"`
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	AccessKeyID     string `json:"accesskeyid"`
	SecretAccessKey string `json:"secretaccesskey"`
	UseSSL          bool   `json:"usessl"`
}

// LoadConfig читает и возвращает готовую структуру конфига
func LoadConfig(path string) (*Config, error) {
	plan, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(plan, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
