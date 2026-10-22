package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// ctxKey - типизированный ключ для передачи конфига через context
// (строковые ключи контекста могут конфликтовать между пакетами)
type ctxKey struct{}

// NewContext сохраняет конфиг в контексте
func NewContext(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, ctxKey{}, cfg)
}

// FromContext достает конфиг из контекста
func FromContext(ctx context.Context) (*Config, error) {
	cfg, ok := ctx.Value(ctxKey{}).(*Config)
	if !ok || cfg == nil {
		return nil, fmt.Errorf("конфигурация не найдена в контексте команды")
	}
	return cfg, nil
}

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
