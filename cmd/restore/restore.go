package cmd

import (
	"fmt"
	"os"

	"kube-dnr/config"

	"github.com/spf13/cobra"
	"kube-dnr/pkg/restore"
	"kube-dnr/pkg/s3"
)

var (
	restoreInput      string
	restoreKubeconfig string
	restoreDownloadS3 bool
	restoreS3Object   string
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Восстановить ресурсы Kubernetes из бэкапа",
	Long: `Восстанавливает ресурсы Kubernetes из архива, созданного командой backup.
Использует KUBECONFIG для подключения к целевому кластеру.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Получаем конфиг из контекста
		cfg := cmd.Context().Value("config").(*config.Config)

		inputPath := restoreInput

		// Если указан флаг download-s3, скачиваем из S3
		if restoreDownloadS3 {
			// Если input не указан, используем последний объект
			if inputPath == "" {
				// Создаем временный файл
				tmpFile := fmt.Sprintf("/tmp/kube-dnr-restore-%d.zip", os.Getpid())
				err := s3.DownloadArchive(cfg, restoreS3Object, tmpFile)
				if err != nil {
					fmt.Printf("Ошибка при скачивании из S3: %v\n", err)
					os.Exit(1)
				}
				inputPath = tmpFile
				defer os.Remove(tmpFile) // Удаляем временный файл после использования
			} else {
				// Если input указан, скачиваем в этот путь
				err := s3.DownloadArchive(cfg, restoreS3Object, inputPath)
				if err != nil {
					fmt.Printf("Ошибка при скачивании из S3: %v\n", err)
					os.Exit(1)
				}
			}
		} else {
			// Без S3 проверяем input
			if inputPath == "" {
				fmt.Println("Ошибка: необходимо указать --input или --download-s3")
				cmd.Help()
				os.Exit(1)
			}
		}

		// Выполняем восстановление
		err := restore.PerformRestore(restoreKubeconfig, inputPath)
		if err != nil {
			fmt.Printf("Ошибка при восстановлении: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Восстановление успешно завершено из: %s\n", inputPath)
	},
}

func NewCommand() *cobra.Command {
	restoreCmd.Flags().StringVarP(&restoreInput, "input", "i", "", "Путь к архиву бэкапа (обязательно, если не используется --download-s3)")
	restoreCmd.Flags().StringVar(&restoreKubeconfig, "kubeconfig", "", "Путь к kubeconfig файлу (по умолчанию ~/.kube/config)")
	restoreCmd.Flags().BoolVar(&restoreDownloadS3, "download-s3", false, "Скачать бэкап из S3")
	restoreCmd.Flags().StringVar(&restoreS3Object, "s3-object", "", "Имя объекта в S3 (если не указано, используется последний)")
	return restoreCmd
}
