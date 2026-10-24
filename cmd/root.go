package cmd

import (
	"context"
	"fmt"
	"os"

	backupcmd "kube-dnr/cmd/backup"
	restorecmd "kube-dnr/cmd/restore"
	"kube-dnr/config"

	"github.com/spf13/cobra"
)

var (
	configPath string

	// version подставляется при сборке через
	// -ldflags "-X kube-dnr/cmd.version=<version>".
	version = "dev"
)

var rootCmd = &cobra.Command{
	Use:   "kube-dnr",
	Short: "Инструмент для бэкапа и восстановления ресурсов Kubernetes",
	Long: `kube-dnr (Kubernetes Disaster Recovery) - это утилита для бэкапа и 
восстановления ресурсов Kubernetes кластера. From Danya With Love❤️

Поддерживаемые режимы:
  backup  - создание бэкапа ресурсов кластера
  restore - восстановление ресурсов из бэкапа`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Проверяем, что указан путь к конфигу
		if configPath == "" {
			return fmt.Errorf("необходимо указать путь к конфигурационному файлу через флаг --config")
		}

		// Загружаем конфиг перед выполнением любой команды
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("ошибка при загрузке конфигурации: %w", err)
		}
		// Сохраняем конфиг в контексте команды
		cmd.SetContext(config.NewContext(context.Background(), cfg))
		return nil
	},
}

func init() {
	rootCmd.Version = version
	rootCmd.SetVersionTemplate(`kube-dnr {{.Version}}
`)

	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Путь к конфигурационному файлу (обязательно)")
	rootCmd.AddCommand(backupcmd.NewCommand())
	rootCmd.AddCommand(restorecmd.NewCommand())
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
