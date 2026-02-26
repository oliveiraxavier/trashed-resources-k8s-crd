package utils

import (
	"context"

	v1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetAllConfigsFromConfigMap(mgr ctrl.Manager, cmName string) v1.ConfigMap {
	ctx := context.Background()
	var cm v1.ConfigMap
	if err := mgr.GetAPIReader().Get(ctx, client.ObjectKey{Namespace: "system", Name: cmName}, &cm); err != nil {
		cm.Data = map[string]string{"kindsTobserve": "Deployment;Secret;ConfigMap"}

	}
	return cm
}
