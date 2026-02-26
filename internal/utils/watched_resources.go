package utils

import (
	"context"
	"strings"

	v1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResourceGVK define a estrutura para mapear o Group e Version de um Kind
type ResourceGVK struct {
	Group   string
	Version string
}

// knownGVKs contém o mapeamento de Kinds comuns para seus respectivos Group e Version
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

func GetKindsToWatch() map[string]ResourceGVK {
	return knownGVKs
}

func GetKindsToWatchFromConfigMap(mgr ctrl.Manager, cmName string) []string {
	// Lê o ConfigMap para descobrir quais recursos observar
	ctx := context.Background()
	var cm v1.ConfigMap
	// Use APIReader em vez de mgr.GetClient() porque o cache ainda não foi iniciado.

	if err := mgr.GetAPIReader().Get(ctx, client.ObjectKey{Namespace: "system", Name: cmName}, &cm); err != nil {
		cm.Data = map[string]string{"kindsTobserve": "Deployment;Secret;ConfigMap"}

	}

	rawKinds := strings.Split(cm.Data["kindsTobserve"], ";")
	return strings.Fields(strings.Join(rawKinds, " ")) // Remove espaços extras
}
