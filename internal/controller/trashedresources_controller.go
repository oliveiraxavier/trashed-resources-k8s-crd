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

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var logger = log.Log
var cmName = "trashedresources-config"

// TrashedResourceReconciler reconciles a trashedresources object
type TrashedResourceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func createUpdatedOrDeletedManifest(c client.Client, kubernetesObj client.Object, action_type string) {
	tr_interactions.CreateOrUpdatedManifest(c, kubernetesObj, action_type)

}

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
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TrashedResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Lê o ConfigMap para descobrir quais recursos observar
	ctx := context.Background()
	var cm v1.ConfigMap
	// Use APIReader em vez de mgr.GetClient() porque o cache ainda não foi iniciado.

	if err := mgr.GetAPIReader().Get(ctx, client.ObjectKey{Namespace: "system", Name: cmName}, &cm); err != nil {
		cm.Data = map[string]string{"kindsTobserve": "Deployment;Secret;ConfigMap"}
		logger.Error(err, "Unable to read ConfigMap, using default values", "kindsTobserve", cm.Data)
	}

	rawKinds := utils.GetKindsToWatchFromConfigMap(mgr, cmName)
	logger.Info("Loading configMap "+cmName+". Kinds found to watch", "kinds", rawKinds)

	builder := ctrl.NewControllerManagedBy(mgr).
		For(&moxv1alpha1.TrashedResource{}).
		WithEventFilter(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool { return false },
			DeleteFunc: func(e event.DeleteEvent) bool {
				if e.Object.GetObjectKind().GroupVersionKind().Kind == "" {
					return false
				}
				tr_interactions.CreateOrUpdatedManifest(mgr.GetClient(), e.Object, "create")
				return true
			},
		}).
		Named("trashedresources")

	getKindsToWatch(mgr, builder) //append kinds to watch based on configmap

	builder = builder.Watches(&moxv1alpha1.TrashedResource{}, &handler.EnqueueRequestForObject{})

	return builder.Complete(r)
}

func getKindsToWatch(mgr ctrl.Manager, builder *ctrl.Builder) *ctrl.Builder {
	logger.Info("Loading configMap " + cmName)
	rawKinds := utils.GetKindsToWatchFromConfigMap(mgr, cmName)

	// For each Kind, add a dynamica watch
	for _, k := range rawKinds {
		kind := strings.TrimSpace(k)
		if kind == "" {
			continue
		}
		knownGVKs := utils.GetKindsToWatch()
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
