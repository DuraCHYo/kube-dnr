package kubeutil

import (
	"os"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// BackupGVRAnnotation - аннотация, в которую при бэкапе записывается GVR ресурса
// (формат "[group/]version/resource"), а при восстановлении читается.
// Позволяет restore не угадывать Resource name по Kind, что важно для CRD.
const BackupGVRAnnotation = "kube-dnr.io/backup-gvr"

// GetRESTConfig получает REST config из kubeconfig
// (если путь не указан, используется ~/.kube/config).
func GetRESTConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		if home := os.Getenv("HOME"); home != "" {
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		}
	}

	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	restConfig.QPS = 50
	restConfig.Burst = 100

	return restConfig, nil
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
