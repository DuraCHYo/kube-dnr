package cmd

import (
	"fmt"
	"os"

	"kube-dnr/config"

	"github.com/spf13/cobra"
	"kube-dnr/pkg/backup"
	"kube-dnr/pkg/s3"
)

var (
	backupAllNamespaces bool
	backupNamespaces    []string
	backupOutput        string
	backupKubeconfig    string
	backupUploadS3      bool
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Создать бэкап ресурсов Kubernetes",
	Long: `Создает бэкап ресурсов Kubernetes в виде манифестов YAML.
Ресурсы сохраняются без метаданных статуса, аннотаций и других полей,
чтобы обеспечить возможность восстановления в другом кластере.`,
	Run: func(cmd *cobra.Command, args []string) {
		if !backupAllNamespaces && len(backupNamespaces) == 0 {
			fmt.Println("Ошибка: необходимо указать либо --all-namespaces (-A), либо --namespaces")
			cmd.Help()
			os.Exit(1)
		}

		if backupOutput == "" {
			fmt.Println("Ошибка: необходимо указать --output")
			cmd.Help()
			os.Exit(1)
		}

		// Определяем неймспейсы для бэкапа
		var namespaces []string
		if backupAllNamespaces {
			namespaces = nil // nil означает все неймспейсы
		} else {
			namespaces = backupNamespaces
		}

		// Получаем конфиг из контекста
		cfg, err := config.FromContext(cmd.Context())
		if err != nil {
			fmt.Printf("Ошибка при получении конфигурации: %v\n", err)
			os.Exit(1)
		}

		// Выполняем бэкап
		if err := backup.PerformBackup(cmd.Context(), backupKubeconfig, namespaces, backupOutput, cfg); err != nil {
			fmt.Printf("Ошибка при создании бэкапа: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Бэкап успешно создан: %s\n", backupOutput)

		// Загружаем в S3 если указан флаг
		if backupUploadS3 {
			if err := s3.UploadArchive(cfg, backupOutput); err != nil {
				fmt.Printf("Ошибка при загрузке в S3: %v\n", err)
				os.Exit(1)
			}
		}
	},
}

func NewCommand() *cobra.Command {
	backupCmd.Flags().BoolVarP(&backupAllNamespaces, "all-namespaces", "A", false, "Бэкапить все неймспейсы")
	backupCmd.Flags().StringSliceVarP(&backupNamespaces, "namespaces", "n", []string{}, "Список неймспейсов для бэкапа (через запятую)")
	backupCmd.Flags().StringVarP(&backupOutput, "output", "o", "", "Путь к выходному архиву (обязательно)")
	backupCmd.Flags().StringVar(&backupKubeconfig, "kubeconfig", "", "Путь к kubeconfig файлу (по умолчанию ~/.kube/config)")
	backupCmd.Flags().BoolVar(&backupUploadS3, "upload-s3", false, "Загрузить бэкап в S3 после создания")
	return backupCmd
}
