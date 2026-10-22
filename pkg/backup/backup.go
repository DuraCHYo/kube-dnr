package backup

import (
	"archive/zip"
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"kube-dnr/config"
	"kube-dnr/pkg/kubeutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	sigsyaml "sigs.k8s.io/yaml"
)

// PerformBackup выполняет бэкап ресурсов Kubernetes
func PerformBackup(ctx context.Context, kubeconfigPath string, namespaces []string, outputPath string, cfg *config.Config) error {
	// Логгер создаем один раз на весь сеанс работы
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	excludedResources := cfg.Kubernetes.ExcludedResources
	excludedSubstrings := cfg.Kubernetes.ExcludedSubstrings
	excludedNamespaces := cfg.Kubernetes.ExcludedNamespaces

	// Получаем REST config
	restConfig, err := kubeutil.GetRESTConfig(kubeconfigPath)
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

	// Список всех неймспейсов получаем один раз лениво, до обхода типов
	// ресурсов, а не для каждого типа отдельно
	var (
		allNamespaces    []string
		allNamespacesGot bool
	)

	// Получаем список всех доступных ресурсов в кластере
	apiResourceLists, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		// Discovery может частично провалиться (недоступные группы API):
		// если данные за какие-то группы все же получены - продолжаем с ними
		if len(apiResourceLists) == 0 {
			return fmt.Errorf("ошибка при получении списка ресурсов: %w", err)
		}
		logger.Warn("Не все группы API доступны, бэкап будет неполным", "err", err)
	}

	// Создаем временный архив
	zipFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("ошибка при создании архива: %w", err)
	}

	zipWriter := zip.NewWriter(zipFile)
	// Закрываем в defer на случай ранних выходов по ошибке;
	// после успешного явного Close повторный Close игнорируем
	defer func() {
		_ = zipWriter.Close()
		_ = zipFile.Close()
	}()

	// Проходим по всем типам ресурсов
	for _, apiResourceList := range apiResourceLists {
		gv, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			logger.Warn("Не удалось парсить GroupVersion", "groupVersion", apiResourceList.GroupVersion, "err", err)
			continue
		}

		for _, apiResource := range apiResourceList.APIResources {
			// Пропускаем ресурсы, которые нельзя листать
			if !slices.Contains(apiResource.Verbs, "list") {
				continue
			}

			// Если Kind ресурса в списке исключенных, пропускаем его до запроса к API
			if slices.Contains(excludedResources, apiResource.Kind) {
				continue
			}

			// Создаем GVR (GroupVersionResource)
			gvr := schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: apiResource.Name,
			}

			// Определяем неймспейсы для бэкапа
			var nsList []string
			if !apiResource.Namespaced {
				// Для кластерных ресурсов используем пустой неймспейс
				nsList = []string{""}
			} else if namespaces != nil {
				nsList = namespaces
			} else {
				// Если указан --all-namespaces, получаем все неймспейсы
				if !allNamespacesGot {
					allNamespaces, err = getAllNamespaces(ctx, clientset)
					if err != nil {
						return fmt.Errorf("ошибка при получении списка неймспейсов: %w", err)
					}
					allNamespacesGot = true
				}
				nsList = allNamespaces
			}

			// Бэкапим ресурсы в каждом неймспейсе
			for _, ns := range nsList {
				// Пропускаем исключенные неймспейсы до запроса
				if ns != "" && slices.Contains(excludedNamespaces, ns) {
					continue
				}

				err := backupResource(ctx, dynamicClient, gvr, ns, apiResource.Kind, excludedSubstrings, zipWriter, logger)
				if err != nil {
					logger.Warn("Ошибка при бэкапе ресурса", "resource", apiResource.Name, "namespace", ns, "err", err)
					continue
				}
			}
		}
	}

	// Финализируем архив явно: центральный каталог ZIP записывается в Close,
	// поэтому не проверить ошибку Close нельзя - иначе архив останется битым
	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("ошибка при записи архива: %w", err)
	}
	if err := zipFile.Close(); err != nil {
		return fmt.Errorf("ошибка при закрытии архива: %w", err)
	}

	return nil
}

// backupResource бэкапит ресурсы определенного типа
func backupResource(ctx context.Context, dynamicClient dynamic.Interface, gvr schema.GroupVersionResource, namespace, kind string, excludedSubstrings []string, zipWriter *zip.Writer, logger *slog.Logger) error {
	// Получаем список ресурсов
	list, err := dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	if len(list.Items) == 0 {
		return nil
	}

	gvrStr := kubeutil.GVRToString(gvr)

	// Проходим по всем ресурсам
	for _, item := range list.Items {
		// Если kind == "Secret" и оно равняется названию неймспейса - пропускаем
		if kind == "Secret" && item.GetName() == namespace {
			continue
		}

		// Проверка на содержащиеся подстроки в имени
		if slices.ContainsFunc(excludedSubstrings, func(sub string) bool {
			return strings.Contains(item.GetName(), sub)
		}) {
			continue
		}

		// Очищаем ресурс от нежелательных полей (модифицируем in place)
		cleanResource(&item)

		// Сохраняем GVR в аннотации: restore использует его вместо
		// эвристики Kind -> Resource name (важно для CRD)
		annotations := item.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[kubeutil.BackupGVRAnnotation] = gvrStr
		item.SetAnnotations(annotations)

		// Конвертируем в YAML
		yamlData, err := sigsyaml.Marshal(item.Object)
		if err != nil {
			return fmt.Errorf("ошибка при кодировании ресурса: %w", err)
		}

		// Создаем путь в архиве
		var filePath string
		if namespace == "" {
			filePath = fmt.Sprintf("cluster-wide/%s/%s.yaml", gvr.Resource, item.GetName())
		} else {
			filePath = fmt.Sprintf("namespaces/%s/%s/%s.yaml", namespace, gvr.Resource, item.GetName())
		}

		// Добавляем файл в архив
		if err := addFileToZip(zipWriter, filePath, yamlData); err != nil {
			return fmt.Errorf("ошибка при добавлении файла в архив: %w", err)
		}
		logger.Info("Файл сохранен", "resource", kind, "file", filePath, "namespace", namespace)
	}
	return nil
}

// cleanResource очищает ресурс от полей, которые не должны попадать в манифест:
// статуса, Runtime-метаданных и иммутабельных полей, для которых API-сервер
// сам генерирует значения. Чистка выполняется на этапе бэкапа, чтобы манифесты
// из архива можно было применять в кластер напрямую, в том числе вручную.
// Модифицирует объект in place, без DeepCopy.
func cleanResource(item *unstructured.Unstructured) {
	// Удаляем поле status
	unstructured.RemoveNestedField(item.Object, "status")

	// Очищаем metadata
	metadata, found, err := unstructured.NestedFieldNoCopy(item.Object, "metadata")
	if found && err == nil {
		if metadataMap, ok := metadata.(map[string]interface{}); ok {
			delete(metadataMap, "uid")
			delete(metadataMap, "selfLink")
			delete(metadataMap, "creationTimestamp")
			delete(metadataMap, "generation")
			delete(metadataMap, "resourceVersion")
			delete(metadataMap, "managedFields")
			delete(metadataMap, "ownerReferences")

			if annotations, ok := metadataMap["annotations"].(map[string]interface{}); ok {
				delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
				delete(annotations, "deployment.kubernetes.io/revision")
				delete(annotations, "kubernetes.io/change-cause")

				if len(annotations) == 0 {
					delete(metadataMap, "annotations")
				}
			}

			if labels, ok := metadataMap["labels"].(map[string]interface{}); ok {
				delete(labels, "pod-template-hash")
				delete(labels, "controller-uid")

				if len(labels) == 0 {
					delete(metadataMap, "labels")
				}
			}
		}
	}

	if item.GetKind() == "ServiceAccount" {
		unstructured.RemoveNestedField(item.Object, "secrets")
	}

	// Удаляю clusterIP/clusterIPs/nodePort из сервиса чтобы не было immutable конфликта
	if item.GetKind() == "Service" {
		unstructured.RemoveNestedField(item.Object, "spec", "clusterIP")
		unstructured.RemoveNestedField(item.Object, "spec", "clusterIPs")
		unstructured.RemoveNestedField(item.Object, "spec", "healthCheckNodePort")

		if ports, found, err := unstructured.NestedFieldNoCopy(item.Object, "spec", "ports"); found && err == nil {
			if portList, ok := ports.([]interface{}); ok {
				for _, port := range portList {
					if portMap, ok := port.(map[string]interface{}); ok {
						delete(portMap, "nodePort")
					}
				}
			}
		}
	}
}

// addFileToZip добавляет файл в ZIP архив
func addFileToZip(zipWriter *zip.Writer, filePath string, data []byte) error {
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
func getAllNamespaces(ctx context.Context, clientset *kubernetes.Clientset) ([]string, error) {
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var nsList []string
	for _, ns := range namespaces.Items {
		nsList = append(nsList, ns.Name)
	}

	return nsList, nil
}
