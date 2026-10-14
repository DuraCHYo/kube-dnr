package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"kube-dnr/pkg/restore"
)

var (
	restoreInput     string
	restoreKubeconfig string
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Восстановить ресурсы Kubernetes из бэкапа",
	Long: `Восстанавливает ресурсы Kubernetes из архива, созданного командой backup.
Использует KUBECONFIG для подключения к целевому кластеру.`,
	Run: func(cmd *cobra.Command, args []string) {
		if restoreInput == "" {
			fmt.Println("Ошибка: необходимо указать --input")
			cmd.Help()
			os.Exit(1)
		}

		// Выполняем восстановление
		err := restore.PerformRestore(restoreKubeconfig, restoreInput)
		if err != nil {
			fmt.Printf("Ошибка при восстановлении: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Восстановление успешно завершено из: %s\n", restoreInput)
	},
}

func NewCommand() *cobra.Command {
	restoreCmd.Flags().StringVarP(&restoreInput, "input", "i", "", "Путь к архиву бэкапа (обязательно)")
	restoreCmd.Flags().StringVar(&restoreKubeconfig, "kubeconfig", "", "Путь к kubeconfig файлу (по умолчанию ~/.kube/config)")
	return restoreCmd
}
