package restore

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"kube-dnr/pkg/kubeutil"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	sigsyaml "sigs.k8s.io/yaml"
)

// PerformRestore выполняет восстановление ресурсов из архива
func PerformRestore(ctx context.Context, kubeconfigPath string, inputPath string) error {
	// Логгер создаем один раз на весь сеанс работы
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Проверяем существование архива
	if _, err := os.Stat(inputPath); err != nil {
		return fmt.Errorf("ошибка при доступе к файлу архива %s: %w", inputPath, err)
	}

	// Получаем REST config
	restConfig, err := kubeutil.GetRESTConfig(kubeconfigPath)
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
			logger.Warn("Не удалось открыть файл", "file", file.Name, "err", err)
			continue
		}

		data, err := io.ReadAll(fileReader)
		fileReader.Close()
		if err != nil {
			logger.Warn("Не удалось прочитать файл", "file", file.Name, "err", err)
			continue
		}

		// Декодируем YAML в Unstructured
		obj := &unstructured.Unstructured{}
		err = sigsyaml.Unmarshal(data, &obj.Object)
		if err != nil {
			logger.Warn("Не удалось декодировать файл", "file", file.Name, "err", err)
			continue
		}

		// Определяем неймспейс из пути файла
		namespace := extractNamespaceFromPath(file.Name)

		// Восстанавливаем ресурс
		err = restoreResource(ctx, dynamicClient, clientset, obj, namespace, logger)
		if err != nil {
			logger.Warn("Ошибка при восстановлении ресурса", "file", file.Name, "err", err)
			continue
		}
		logger.Info("Восстановлен", "kind", obj.GetKind(), "object", obj.GetName(), "namespace", namespace)
	}

	return nil
}

// restoreResource восстанавливает отдельный ресурс
func restoreResource(ctx context.Context, dynamicClient dynamic.Interface, clientset *kubernetes.Clientset, obj *unstructured.Unstructured, namespace string, logger *slog.Logger) error {
	gvk := obj.GroupVersionKind()

	// Приоритет: GVR из аннотации бэкапа, затем эвристика Kind -> Resource name
	gvr := gvrFromBackup(obj)

	// Для кластерных ресурсов используем пустой неймспейс
	if namespace == "" {
		namespace = obj.GetNamespace()
	}

	// Убираем служебную аннотацию перед отправкой в кластер
	removeBackupAnnotation(obj)

	// Проверяем, существует ли ресурс
	_, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, obj.GetName(), metav1.GetOptions{})
	switch {
	case err == nil:
		// Ресурс существует
		if reason, immutable := immutableSpecKinds[gvk.Kind]; immutable {
			// У таких ресурсов spec immutable: update существующего объекта будет ошибка, скипаем
			logger.Warn("Ресурс существует, обновление пропущено",
				"kind", gvk.Kind,
				"name", obj.GetName(),
				"namespace", namespace,
				"reason", reason)
			return nil
		}

		// Обновляем
		_, err = dynamicClient.Resource(gvr).Namespace(namespace).Update(ctx, obj, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("ошибка при обновлении ресурса: %w", err)
		}

	case apierrors.IsNotFound(err):
		// Ресурс не существует - создаем. Сначала создаем неймспейс если нужно
		if namespace != "" && gvk.Kind != "Namespace" {
			err = ensureNamespaceExists(ctx, clientset, namespace)
			if err != nil {
				return fmt.Errorf("ошибка при создании неймспейса %s: %w", namespace, err)
			}
		}

		_, err = dynamicClient.Resource(gvr).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("ошибка при создании ресурса: %w", err)
		}

	default:
		// Любая ошибка кроме NotFound - выводим
		return fmt.Errorf("ошибка при проверке существования ресурса: %w", err)
	}

	return nil
}

var immutableSpecKinds = map[string]string{
	"PersistentVolume":      "spec иммутабелен",
	"PersistentVolumeClaim": "spec иммутабелен (кроме увеличения размера диска)",
	"Pod":                   "spec иммутабелен (кроме ephemeral containers)",
	"Job":                   "spec иммутабелен (кроме suspend/parallelism)",
}

// ensureNamespaceExists создает неймспейс если он не существует
func ensureNamespaceExists(ctx context.Context, clientset *kubernetes.Clientset, namespace string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}

	// Создаем неймспейс; если он уже существует (в том числе из-за гонки) - это не ошибка
	_, err := clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// gvrFromBackup определяет GVR ресурса: приоритет у аннотации из бэкапа, затем резервный вариант на kind => ResourceName
func gvrFromBackup(obj *unstructured.Unstructured) schema.GroupVersionResource {
	if gvrStr := obj.GetAnnotations()[kubeutil.BackupGVRAnnotation]; gvrStr != "" {
		if gvr, ok := kubeutil.GVRFromString(gvrStr); ok {
			return gvr
		}
	}

	gvk := obj.GroupVersionKind()
	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: getResourceNameFromKind(gvk.Kind),
	}
}

// removeBackupAnnotation удаляет служебную аннотацию бэкапа из ресурса
func removeBackupAnnotation(obj *unstructured.Unstructured) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return
	}

	delete(annotations, kubeutil.BackupGVRAnnotation)
	if len(annotations) == 0 {
		annotations = nil
	}
	obj.SetAnnotations(annotations)
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
	// Используется как fallback, если в манифесте нет аннотации с GVR
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
