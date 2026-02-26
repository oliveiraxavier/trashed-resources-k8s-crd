package utils

import (
	"strings"

	v1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ResourceGVK define a estrutura para mapear o Group e Version de um Kind
type ResourceGVK struct {
	Group   string
	Version string
}

var knownGVKs = map[string]ResourceGVK{
	"deployment":  {Group: "apps", Version: "v1"},
	"secret":      {Group: "", Version: "v1"},
	"configmap":   {Group: "", Version: "v1"},
	"statefulset": {Group: "apps", Version: "v1"},
	"daemonset":   {Group: "apps", Version: "v1"},
	"ingress":     {Group: "networking.k8s.io", Version: "v1"},
	"cronjob":     {Group: "batch", Version: "v1"},
	"job":         {Group: "batch", Version: "v1"},
	"service":     {Group: "", Version: "v1"},
}

func GetKnownKindsToWatch() map[string]ResourceGVK {
	return knownGVKs
}

func GetKindsToWatchFromConfigMap(mgr ctrl.Manager, configMapData v1.ConfigMap, cmName string) []string {
	rawKinds := strings.Split(configMapData.Data["kindsTobserve"], ";")

	return strings.Fields(strings.Join(rawKinds, " "))
}
