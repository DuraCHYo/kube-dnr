package restore

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	sigsyaml "sigs.k8s.io/yaml"
)

// PerformRestore выполняет восстановление ресурсов из архива
func PerformRestore(kubeconfigPath string, inputPath string) error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	// Проверяем существование архива
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return fmt.Errorf("файл архива не существует: %s", inputPath)
	}

	// Получаем REST config
	restConfig, err := getRESTConfig(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("ошибка при получении конфигурации: %w", err)
	}

	// Создаем clients
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("ошибка при создании dynamic client: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("ошибка при создании клиента Kubernetes: %w", err)
	}

	// Открываем архив
	zipReader, err := zip.OpenReader(inputPath)
	if err != nil {
		return fmt.Errorf("ошибка при открытии архива: %w", err)
	}
	defer zipReader.Close()

	// Проходим по всем файлам в архиве
	for _, file := range zipReader.File {
		// Пропускаем директории
		if file.FileInfo().IsDir() {
			continue
		}

		// Читаем файл
		fileReader, err := file.Open()
		if err != nil {
			fmt.Printf("Предупреждение: не удалось открыть файл %s: %v\n", file.Name, err)
			continue
		}

		data, err := io.ReadAll(fileReader)
		fileReader.Close()
		if err != nil {
			fmt.Printf("Предупреждение: не удалось прочитать файл %s: %v\n", file.Name, err)
			continue
		}

		// Декодируем YAML в Unstructured
		obj := &unstructured.Unstructured{}
		err = sigsyaml.Unmarshal(data, &obj.Object)
		if err != nil {
			fmt.Printf("Предупреждение: не удалось декодировать файл %s: %v\n", file.Name, err)
			continue
		}

		// Определяем неймспейс из пути файла
		namespace := extractNamespaceFromPath(file.Name)

		// Восстанавливаем ресурс
		err = restoreResource(dynamicClient, clientset, obj, namespace)
		if err != nil {
			fmt.Printf("Предупреждение: ошибка при восстановлении ресурса из %s: %v\n", file.Name, err)
			continue
		}
		logger.Info("Восстановлен", "kind", obj.GetKind(), "object", obj.GetName(), "namespace", namespace)
	}

	return nil
}

// restoreResource восстанавливает отдельный ресурс
func restoreResource(dynamicClient dynamic.Interface, clientset *kubernetes.Clientset, obj *unstructured.Unstructured, namespace string) error {
	gvk := obj.GroupVersionKind()
	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: getResourceNameFromKind(gvk.Kind),
	}

	// Для кластерных ресурсов используем пустой неймспейс
	if namespace == "" {
		namespace = obj.GetNamespace()
	}

	// Проверяем, существует ли ресурс
	_, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), obj.GetName(), metav1.GetOptions{})
	if err == nil {
		// Ресурс существует - обновляем
		_, err = dynamicClient.Resource(gvr).Namespace(namespace).Update(context.TODO(), obj, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("ошибка при обновлении ресурса: %w", err)
		}
	} else {
		// Ресурс не существует - создаем
		// Сначала создаем неймспейс если нужно
		if namespace != "" && gvk.Kind != "Namespace" {
			err = ensureNamespaceExists(clientset, namespace)
			if err != nil {
				return fmt.Errorf("ошибка при создании неймспейса %s: %w", namespace, err)
			}
		}

		_, err = dynamicClient.Resource(gvr).Namespace(namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("ошибка при создании ресурса: %w", err)
		}
	}

	return nil
}

// ensureNamespaceExists создает неймспейс если он не существует
func ensureNamespaceExists(clientset *kubernetes.Clientset, namespace string) error {
	_, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{})
	if err == nil {
		return nil // Неймспейс уже существует
	}

	// Создаем неймспейс
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}

	_, err = clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// extractNamespaceFromPath извлекает неймспейс из пути файла в архиве
func extractNamespaceFromPath(path string) string {
	// Формат пути: namespaces/{namespace}/{resource}/{name}.yaml или cluster-wide/{resource}/{name}.yaml
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return ""
	}

	if parts[0] == "namespaces" {
		return parts[1]
	}

	return "" // cluster-wide ресурс
}

// getResourceNameFromKind конвертирует Kind в Resource name
func getResourceNameFromKind(kind string) string {
	// Простое преобразование: Pod -> pods, Deployment -> deployments
	// Для более точного преобразования можно использовать discovery API
	kindLower := strings.ToLower(kind)

	// Специальные случаи
	switch kind {
	case "Endpoints":
		return "endpoints"
	case "Node":
		return "nodes"
	case "Namespace":
		return "namespaces"
	case "PersistentVolume":
		return "persistentvolumes"
	case "PersistentVolumeClaim":
		return "persistentvolumeclaims"
	case "ServiceAccount":
		return "serviceaccounts"
	case "ConfigMap":
		return "configmaps"
	case "Secret":
		return "secrets"
	case "Ingress":
		return "ingresses"
	case "NetworkPolicy":
		return "networkpolicies"
	case "PodDisruptionBudget":
		return "poddisruptionbudgets"
	case "StatefulSet":
		return "statefulsets"
	case "DaemonSet":
		return "daemonsets"
	case "ReplicaSet":
		return "replicasets"
	case "Job":
		return "jobs"
	case "CronJob":
		return "cronjobs"
	}

	// По умолчанию добавляем 's' если заканчивается не на 's'
	if strings.HasSuffix(kindLower, "s") {
		return kindLower
	}
	return kindLower + "s"
}

// getRESTConfig получает REST config из kubeconfig
func getRESTConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		if home := os.Getenv("HOME"); home != "" {
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		}
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	config.QPS = 50
	config.Burst = 100

	return config, nil
}
