package kubeutil

import (
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// BackupGVRAnnotation - аннотация, в которую при бэкапе записывается GVR ресурса
// (формат "[group/]version/resource"), а при восстановлении читается.
// Позволяет restore не угадывать Resource name по Kind, что важно для CRD.
const BackupGVRAnnotation = "kube-dnr.io/backup-gvr"

// GetRESTConfig возвращает конфигурацию для подключения к Kubernetes.
// Приоритет источников:
//  1. явно указанный kubeconfig (флаг --kubeconfig);
//  2. ServiceAccount, если программа запущена внутри пода Kubernetes;
//  3. kubeconfig из $KUBECONFIG или ~/.kube/config.
func GetRESTConfig(kubeconfigPath string) (*rest.Config, error) {
	restConfig, err := resolveRESTConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	restConfig.QPS = 50
	restConfig.Burst = 100

	return restConfig, nil
}

func resolveRESTConfig(kubeconfigPath string) (*rest.Config, error) {
	// Явно указанный kubeconfig имеет приоритет даже внутри кластера.
	if kubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}

	// Внутри пода используем ServiceAccount (KUBERNETES_SERVICE_HOST/PORT
	// выставлены, токен смонтирован в /var/run/secrets/...).
	restConfig, err := rest.InClusterConfig()
	if err == nil {
		return restConfig, nil
	}
	if !errors.Is(err, rest.ErrNotInCluster) {
		return nil, fmt.Errorf("ошибка in-cluster конфигурации: %w", err)
	}

	// Вне кластера — обычный kubeconfig.
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
}

// GVRToString форматирует GVR в строку "[group/]version/resource".
func GVRToString(gvr schema.GroupVersionResource) string {
	if gvr.Group == "" {
		return gvr.Version + "/" + gvr.Resource
	}
	return gvr.Group + "/" + gvr.Version + "/" + gvr.Resource
}

// GVRFromString разбирает строку формата "[group/]version/resource".
func GVRFromString(s string) (schema.GroupVersionResource, bool) {
	parts := strings.SplitN(s, "/", 3)
	switch len(parts) {
	case 2:
		return schema.GroupVersionResource{Version: parts[0], Resource: parts[1]}, true
	case 3:
		return schema.GroupVersionResource{Group: parts[0], Version: parts[1], Resource: parts[2]}, true
	}
	return schema.GroupVersionResource{}, false
}
