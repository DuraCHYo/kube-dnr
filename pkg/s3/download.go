package s3

import (
	"context"
	"fmt"
	"io"
	"kube-dnr/config"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// DownloadArchive скачивает архив из S3
func DownloadArchive(cfg *config.Config, objectName string, outputPath string) error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()

	endpoint := cfg.S3.Endpoint
	accessKeyID := cfg.S3.AccessKeyID
	secretAccessKey := cfg.S3.SecretAccessKey
	useSSL := cfg.S3.UseSSL
	bucketName := cfg.S3.Bucket

	// Создаем клиент MinIO
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return fmt.Errorf("ошибка при создании S3 клиента: %w", err)
	}

	// Проверяем существование бакета
	exists, err := minioClient.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("ошибка при проверке бакета: %w", err)
	}

	if !exists {
		return fmt.Errorf("бакет не существует: %s", bucketName)
	}

	// Если objectName пустой, получаем последний файл
	if objectName == "" {
		objectName, err = getLatestObject(ctx, minioClient, bucketName)
		if err != nil {
			return fmt.Errorf("ошибка при получении последнего объекта: %w", err)
		}
		logger.Info("Используем последний объект", "object", objectName)
	}

	// Проверяем существование объекта
	_, err = minioClient.StatObject(ctx, bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		return fmt.Errorf("объект не существует: %s", objectName)
	}

	// Создаем директорию для файла если нужно
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("ошибка при создании директории: %w", err)
		}
	}

	// Скачиваем объект
	err = minioClient.FGetObject(ctx, bucketName, objectName, outputPath, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("ошибка при скачивании файла: %w", err)
	}

	logger.Info("Файл успешно скачан из S3",
		"object", objectName,
		"path", outputPath,
		"bucket", bucketName)

	return nil
}

// getLatestObject получает последний объект из бакета
func getLatestObject(ctx context.Context, client *minio.Client, bucketName string) (string, error) {
	// Создаем канал для объектов
	objectCh := client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{})

	var latestObject string
	var latestTime int64 = 0

	for object := range objectCh {
		if object.Err != nil {
			return "", object.Err
		}

		// Ищем объект с последним временем модификации
		if object.LastModified.Unix() > latestTime {
			latestTime = object.LastModified.Unix()
			latestObject = object.Key
		}
	}

	if latestObject == "" {
		return "", fmt.Errorf("бакет пуст")
	}

	return latestObject, nil
}

// ListObjects возвращает список объектов в бакете
func ListObjects(cfg *config.Config) ([]string, error) {
	ctx := context.Background()

	endpoint := cfg.S3.Endpoint
	accessKeyID := cfg.S3.AccessKeyID
	secretAccessKey := cfg.S3.SecretAccessKey
	useSSL := cfg.S3.UseSSL
	bucketName := cfg.S3.Bucket

	// Создаем клиент MinIO
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("ошибка при создании S3 клиента: %w", err)
	}

	// Проверяем существование бакета
	exists, err := minioClient.BucketExists(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("ошибка при проверке бакета: %w", err)
	}

	if !exists {
		return nil, fmt.Errorf("бакет не существует: %s", bucketName)
	}

	// Получаем список объектов
	objectCh := minioClient.ListObjects(ctx, bucketName, minio.ListObjectsOptions{})

	var objects []string
	for object := range objectCh {
		if object.Err != nil {
			return nil, object.Err
		}
		objects = append(objects, object.Key)
	}

	return objects, nil
}

// DownloadObject скачивает объект в Reader
func DownloadObject(cfg *config.Config, objectName string) (io.ReadCloser, error) {
	ctx := context.Background()

	endpoint := cfg.S3.Endpoint
	accessKeyID := cfg.S3.AccessKeyID
	secretAccessKey := cfg.S3.SecretAccessKey
	useSSL := cfg.S3.UseSSL
	bucketName := cfg.S3.Bucket

	// Создаем клиент MinIO
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("ошибка при создании S3 клиента: %w", err)
	}

	// Получаем объект
	object, err := minioClient.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении объекта: %w", err)
	}

	return object, nil
}
