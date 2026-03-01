/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"strings"
	moxv1alpha1 "trashed-resources/api/v1alpha1"
	tr_interactions "trashed-resources/internal/domain/trashedresources"
	utils "trashed-resources/internal/utils"

	"slices"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var (
	logger = log.Log
	cmName = "trashedresources-config"
)

// TrashedResourceReconciler wraps the common reconciler to allow defining methods in this package.
type TrashedResourceReconciler utils.TrashedResourceReconciler

// +kubebuilder:rbac:groups=mox.app.br,resources=trashedresources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mox.app.br,resources=trashedresources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mox.app.br,resources=trashedresources/finalizers,verbs=update
// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the trashedresources object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *TrashedResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Example usage: Accessing the config map data stored in the reconciler struct
	logger.Info("Reconciling with config", "config", r.Config.Data)
	logger.Info("Reconciling with config 	KindsToWatch", "kindsToWatch", r.KindsToWatch)
	logger.Info("Reconciling with config 	ActionsToWatch", "actionsToWatch", r.ActionsToWatch)
	logger.Info("Reconciling with config 	NamespacesToIgnore", "namespacesToIgnore", r.NamespacesToIgnore)

	// 1. Fetch the TrashedResource instance
	trashedResource, err := tr_interactions.GetToReconcile(ctx, r.Client, req.Name, req.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	if trashedResource == nil {
		// Resource not found (deleted), stop reconciliation
		return ctrl.Result{}, nil
	}

	// 2. Check expiration and delete if expired
	timeRemaining := utils.GetTimeRemaining(trashedResource.Spec.KeepUntil)
	if timeRemaining <= 0 {
		logger.Info("TrashedResource expired, deleting", "name", req.Name, "namespace", req.Namespace)
		return ctrl.Result{}, tr_interactions.DeleteToReconcile(ctx, r.Client, req.Name, req.Namespace)
	}

	// 3. Requeue after the remaining time
	return ctrl.Result{Requeue: true, RequeueAfter: timeRemaining}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TrashedResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Config = utils.GetAllConfigsFromConfigMap(mgr, cmName)
	r.KindsToWatch = utils.GetKindsToWatchFromConfigMap(r.Config)
	r.ActionsToWatch = utils.GetActionsToWatchFromConfigMap(r.Config)
	r.NamespacesToIgnore = utils.GetNamespacesToIgnoreFromConfigMap(r.Config)
	r.MinutesToKeep = utils.GetMinutesToKeepFromConfigMap(r.Config)
	r.HoursToKeep = utils.GetHoursToKeepFromConfigMap(r.Config)
	r.DaysToKeep = utils.GetDaysToKeepFromConfigMap(r.Config)

	logger.Info("Kinds found to watch", "kinds", r.KindsToWatch)
	logger.Info("Actions found to watch", "actions", r.ActionsToWatch)
	logger.Info("Namespaces to ignore", "namespaces", r.NamespacesToIgnore)
	logger.Info("Minutes to keep", "minutes", r.MinutesToKeep)
	logger.Info("Hours to keep", "hours", r.HoursToKeep)
	logger.Info("Days to keep", "days", r.DaysToKeep)

	builder := ctrl.NewControllerManagedBy(mgr).
		For(&moxv1alpha1.TrashedResource{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				keyExists := slices.Contains(r.ActionsToWatch, "update")
				ignoreNamespace := slices.Contains(r.NamespacesToIgnore, e.ObjectOld.GetNamespace()) ||
					slices.Contains(r.NamespacesToIgnore, e.ObjectNew.GetNamespace())

				if e.ObjectOld.GetObjectKind().GroupVersionKind().Kind == "" ||
					e.ObjectOld.GetGeneration() == e.ObjectNew.GetGeneration() || // Ignore status updates
					!keyExists || ignoreNamespace {
					return false
				}
				logger.Info("Update event detected", "name", e.ObjectOld.GetName(), "namespace", e.ObjectOld.GetNamespace())
				tr_interactions.CreateOrUpdatedManifest(mgr.GetClient(), e.ObjectOld, (*tr_interactions.TRReconciler)(r), "updated")
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				keyExists := slices.Contains(r.ActionsToWatch, "delete")
				ignoreNamespace := slices.Contains(r.NamespacesToIgnore, e.Object.GetNamespace())

				if e.Object.GetObjectKind().GroupVersionKind().Kind == "" || (!keyExists || ignoreNamespace) {
					return false
				}
				logger.Info("Delete event detected", "name", e.Object.GetName(), "namespace", e.Object.GetNamespace())
				tr_interactions.CreateOrUpdatedManifest(mgr.GetClient(), e.Object, (*tr_interactions.TRReconciler)(r), "deleted")
				return false
			},
		}).
		Named("trashedresources")

	// Para cada Kind configurado, adiciona um Watch.
	// Os eventos desses watches serão filtrados pelos predicados definidos acima.
	// A lógica de criação do TrashedResource acontece DENTRO dos predicados.
	appendKindsToWatch(mgr, builder, r.Config)

	return builder.Complete(r)
}

func appendKindsToWatch(mgr ctrl.Manager, builder *ctrl.Builder, configMapData v1.ConfigMap) *ctrl.Builder {
	rawKinds := utils.GetKindsToWatchFromConfigMap(configMapData)

	// For each Kind, add a dynamica watch
	for _, k := range rawKinds {
		kind := strings.TrimSpace(k)
		if kind == "" {
			continue
		}
		knownGVKs := utils.GetKnownKindsToWatch()
		// Busca o GVK correto no mapa (case-insensitive lookup)
		rgvk, ok := knownGVKs[strings.ToLower(kind)]
		if !ok {
			logger.Info("Kind not explicitly mapped; ignoring.", "kind", kind)
			logger.Info("Kinds mapped are:", "kinds", knownGVKs)
			continue
		}

		gvk := schema.GroupVersionKind{
			Group:   rgvk.Group,
			Version: rgvk.Version,
			Kind:    kind,
		}
		logger.Info("Watching kind", "kind", kind)
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		builder = builder.Watches(u, &handler.EnqueueRequestForObject{})
	}

	return builder
}
