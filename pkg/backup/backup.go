package backup

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	sigsyaml "sigs.k8s.io/yaml"
)

// PerformBackup выполняет бэкап ресурсов Kubernetes
func PerformBackup(kubeconfigPath string, namespaces []string, outputPath string) error {
	// Получаем REST config
	restConfig, err := getRESTConfig(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("ошибка при получении конфигурации: %w", err)
	}

	// Получаем клиент Kubernetes
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("ошибка при создании клиента Kubernetes: %w", err)
	}

	// Создаем dynamic client для работы с любыми ресурсами
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("ошибка при создании dynamic client: %w", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("ошибка при создании discovery client: %w", err)
	}

	// Получаем список всех доступных ресурсов в кластере
	apiResourceLists, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		return fmt.Errorf("ошибка при получении списка ресурсов: %w", err)
	}

	// Создаем временный архив
	zipFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("ошибка при создании архива: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Проходим по всем типам ресурсов
	for _, apiResourceList := range apiResourceLists {
		gv, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			fmt.Printf("Предупреждение: не удалось парсить GroupVersion %s: %v\n", apiResourceList.GroupVersion, err)
			continue
		}

		for _, apiResource := range apiResourceList.APIResources {
			// Пропускаем ресурсы, которые нельзя листать
			if !contains(apiResource.Verbs, "list") {
				continue
			}

			// Создаем GVR (GroupVersionResource)
			gvr := schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: apiResource.Name,
			}

			// Определяем неймспейсы для бэкапа
			nsList := namespaces
			if !apiResource.Namespaced {
				// Для кластерных ресурсов используем пустой неймспейс
				nsList = []string{""}
			} else if namespaces == nil {
				// Если указан --all-namespaces, получаем все неймспейсы
				nsList, err = getAllNamespaces(clientset)
				if err != nil {
					return fmt.Errorf("ошибка при получении списка неймспейсов: %w", err)
				}
			}

			// Бэкапим ресурсы в каждом неймспейсе
			for _, ns := range nsList {
				err := backupResource(dynamicClient, gvr, ns, apiResource.Kind, zipWriter)
				if err != nil {
					fmt.Printf("Предупреждение: ошибка при бэкапе %s в неймспейсе %s: %v\n", apiResource.Name, ns, err)
					continue
				}
			}
		}
	}

	return nil
}

// backupResource бэкапит ресурсы определенного типа
func backupResource(dynamicClient dynamic.Interface, gvr schema.GroupVersionResource, namespace, kind string, zipWriter *zip.Writer) error {
	// Получаем список ресурсов
	list, err := dynamicClient.Resource(gvr).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	if len(list.Items) == 0 {
		return nil
	}

	// Проходим по всем ресурсам
	for _, item := range list.Items {
		// Очищаем ресурс от нежелательных полей
		cleaned := cleanResource(&item)

		// Конвертируем в YAML
		yamlData, err := sigsyaml.Marshal(cleaned.Object)
		if err != nil {
			return fmt.Errorf("ошибка при кодировании ресурса: %w", err)
		}

		// Создаем путь в архиве
		var filePath string
		if namespace == "" {
			filePath = fmt.Sprintf("cluster-wide/%s/%s.yaml", gvr.Resource, cleaned.GetName())
		} else {
			filePath = fmt.Sprintf("namespaces/%s/%s/%s.yaml", namespace, gvr.Resource, cleaned.GetName())
		}

		// Добавляем файл в архив
		err = addFileToZip(zipWriter, filePath, yamlData)
		if err != nil {
			return fmt.Errorf("ошибка при добавлении файла в архив: %w", err)
		}
	}

	return nil
}

// cleanResource очищает ресурс от метаданных статуса и других полей
func cleanResource(item *unstructured.Unstructured) *unstructured.Unstructured {
	cleaned := item.DeepCopy()

	// Удаляем поле status
	unstructured.RemoveNestedField(cleaned.Object, "status")

	// Очищаем metadata
	metadata, found, err := unstructured.NestedFieldNoCopy(cleaned.Object, "metadata")
	if found && err == nil {
		if metadataMap, ok := metadata.(map[string]interface{}); ok {
			// Удаляем поля которые не нужны для восстановления
			delete(metadataMap, "uid")
			delete(metadataMap, "selfLink")
			delete(metadataMap, "creationTimestamp")
			delete(metadataMap, "generation")
			delete(metadataMap, "resourceVersion")
			delete(metadataMap, "managedFields")
			delete(metadataMap, "ownerReferences")

			// Очищаем annotations
			if annotations, ok := metadataMap["annotations"].(map[string]interface{}); ok {
				// Удаляем системные аннотации
				delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
				delete(annotations, "deployment.kubernetes.io/revision")
				delete(annotations, "kubernetes.io/change-cause")

				// Если после очистки annotations пуст, удаляем поле
				if len(annotations) == 0 {
					delete(metadataMap, "annotations")
				}
			}

			// Очищаем labels - можно удалить специфичные системные лейблы
			if labels, ok := metadataMap["labels"].(map[string]interface{}); ok {
				delete(labels, "pod-template-hash")
				delete(labels, "controller-uid")

				// Если после очистки labels пуст, удаляем поле
				if len(labels) == 0 {
					delete(metadataMap, "labels")
				}
			}

			// Обновляем metadata
			cleaned.Object["metadata"] = metadataMap
		}
	}

	// Очищаем специфичные поля для разных типов ресурсов
	// Например, для ServiceAccount удаляем secrets
	if cleaned.GetKind() == "ServiceAccount" {
		unstructured.RemoveNestedField(cleaned.Object, "secrets")
	}

	return cleaned
}

// addFileToZip добавляет файл в ZIP архив
func addFileToZip(zipWriter *zip.Writer, filePath string, data []byte) error {
	// Создаем директории в архиве если нужно
	header := &zip.FileHeader{
		Name:   filePath,
		Method: zip.Deflate,
	}

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = writer.Write(data)
	return err
}

// getAllNamespaces получает список всех неймспейсов
func getAllNamespaces(clientset *kubernetes.Clientset) ([]string, error) {
	namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var nsList []string
	for _, ns := range namespaces.Items {
		nsList = append(nsList, ns.Name)
	}

	return nsList, nil
}

// contains проверяет наличие строки в слайсе
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
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
