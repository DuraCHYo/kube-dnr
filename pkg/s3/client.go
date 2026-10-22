package s3

import (
	"fmt"

	"kube-dnr/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// newClient создает MinIO клиент из конфигурации S3
func newClient(cfg *config.Config) (*minio.Client, error) {
	client, err := minio.New(cfg.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3.AccessKeyID, cfg.S3.SecretAccessKey, ""),
		Secure: cfg.S3.UseSSL,
		Region: cfg.S3.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("ошибка при создании S3 клиента: %w", err)
	}

	return client, nil
}
