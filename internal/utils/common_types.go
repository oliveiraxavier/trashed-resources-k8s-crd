package utils

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TrashedResourceReconciler reconciles a trashedresources object
type TrashedResourceReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	Config             v1.ConfigMap
	KindsToWatch       []string
	ActionsToWatch     []string
	NamespacesToIgnore []string
	MinutesToKeep      string
	HoursToKeep        string
	DaysToKeep         string
}
