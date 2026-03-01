package utils

import (
	"context"
	"strconv"

	v1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	ctx    = context.Background()
	logger = log.Log
)

type TRReconciler TrashedResourceReconciler

func GetAllConfigsFromConfigMap(mgr ctrl.Manager, cmName string) v1.ConfigMap {
	var cm v1.ConfigMap
	logger.Info("Loading configMap " + cmName)

	// Use APIReader instead of mgr.GetClient() because the cache is not yet initialized.
	if err := mgr.GetAPIReader().Get(ctx, client.ObjectKey{Namespace: "system", Name: cmName}, &cm); err != nil {
		cm.Data = map[string]string{
			"kindsTobserve":      "Deployment;Secret;ConfigMap",
			"actionsToObserve":   "delete",
			"namespacesToIgnore": "istio-system; kube-node-lease; kube-public; kube-system",
			"minutesToKeep":      "60",
			"hoursToKeep":        "0",
			"daysToKeep":         "0",
		}
	}
	return cm
}

func GetTimetoKeepFromConfigMap(resourceReconciler *TRReconciler) string {
	minutesToKeep := int64(60)         // default value
	hoursToKeep := int64(0)            // default value
	daysToKeepInHours := int64(0) * 24 // default value is 0, but converted to hours
	dateNow := Now()

	if mtk, err := strconv.Atoi(resourceReconciler.MinutesToKeep); err == nil {
		minutesToKeep = int64(mtk)
	} else {
		logger.Error(err, "Invalid value for minutesToKeep in ConfigMap, using default", "minutesToKeep", resourceReconciler.MinutesToKeep)
	}

	if htk, err := strconv.Atoi(resourceReconciler.HoursToKeep); err == nil {
		hoursToKeep = int64(htk)
	} else {
		logger.Error(err, "Invalid value for hoursToKeep in ConfigMap, using default", "hoursToKeep", resourceReconciler.HoursToKeep)
	}

	if dtk, err := strconv.Atoi(resourceReconciler.DaysToKeep); err == nil {
		daysToKeepInHours = int64(dtk) * 24 // Convert days to hours
	} else {
		logger.Error(err, "Invalid value for daysToKeep in ConfigMap, using default", "daysToKeep", resourceReconciler.DaysToKeep)
	}

	return dateNow.AddMinutes(minutesToKeep).AddHours(hoursToKeep).AddHours(daysToKeepInHours).ToString()
}
