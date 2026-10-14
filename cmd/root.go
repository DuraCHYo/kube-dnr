package cmd

import (
	"os"

	backupcmd "kube-dnr/cmd/backup"
	restorecmd "kube-dnr/cmd/restore"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "kube-dnr",
	Short: "Инструмент для бэкапа и восстановления ресурсов Kubernetes",
	Long: `kube-dnr (Kubernetes Disaster Recovery) - это утилита для бэкапа и 
восстановления ресурсов Kubernetes кластера. From Danya With Love❤️

Поддерживаемые режимы:
  backup  - создание бэкапа ресурсов кластера
  restore - восстановление ресурсов из бэкапа`,
}

func init() {
	rootCmd.AddCommand(backupcmd.NewCommand())
	rootCmd.AddCommand(restorecmd.NewCommand())
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
