package s3

import (
	"context"
	"fmt"
	"kube-dnr/config"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/minio/minio-go/v7"
)

// UploadArchive загружает архив в S3
func UploadArchive(cfg *config.Config, filePath string) error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()

	// Проверяем существование файла
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("файл не существует: %s", filePath)
	}

	bucketName := cfg.S3.Bucket
	location := cfg.S3.Region

	// Создаем клиент MinIO
	minioClient, err := newClient(cfg)
	if err != nil {
		return err
	}

	// Проверяем существование бакета
	exists, err := minioClient.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("ошибка при проверке бакета: %w", err)
	}

	if !exists {
		// Создаем бакет если не существует
		err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: location})
		if err != nil {
			return fmt.Errorf("ошибка при создании бакета: %w", err)
		}
		logger.Info("Бакет создан", "bucket", bucketName)
	} else {
		logger.Info("Используем существующий бакет", "bucket", bucketName)
	}

	// Генерируем имя объекта на основе текущего времени и имени файла
	timestamp := time.Now().Format("20060102-150405")
	fileName := filepath.Base(filePath)
	objectName := fmt.Sprintf("%s/%s", timestamp, fileName)

	// Загружаем файл
	contentType := "application/zip"
	info, err := minioClient.FPutObject(ctx, bucketName, objectName, filePath, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("ошибка при загрузке файла: %w", err)
	}

	logger.Info("Файл успешно загружен в S3",
		"object", objectName,
		"size", info.Size,
		"bucket", bucketName)

	return nil
}
