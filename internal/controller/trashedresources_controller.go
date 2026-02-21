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

	moxv1alpha1 "trashed-resources/api/v1alpha1"

	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// TrashedResourceReconciler reconciles a trashedresources object
type TrashedResourceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// logPodManifestDelete loga o manifesto YAML do Pod deletado
func getUpdatedOrDeletedManifest(c client.Client, kubernetesObj client.Object) {
	ctx := context.Background()
	podJSON, err := json.Marshal(kubernetesObj)
	if err != nil {
		log.Log.Error(err, "Erro ao serializar o objeto para JSON")
		return
	}
	var podMap map[string]interface{}
	if err := json.Unmarshal(podJSON, &podMap); err != nil {
		log.Log.Error(err, "Erro ao desserializar JSON do objeto")
		return
	}
	// Remove campos gerenciados para simplificar o YAML
	podMap["metadata"].(map[string]interface{})["managedFields"] = nil
	podYAML, err := yaml.Marshal(podMap)
	if err != nil {
		log.Log.Error(err, "Erro ao serializar objeto para YAML")
		return
	}
	log.Log.Info("Manifesto do objeto deletado:", "yaml", kubernetesObj.GetName())

	// Cria o TrashedResource
	trashed := &moxv1alpha1.TrashedResource{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("trashed-%s-%s-", kubernetesObj.GetObjectKind().GroupVersionKind().Kind, kubernetesObj.GetName()),
			Namespace:    kubernetesObj.GetNamespace(),
		},
		Spec: moxv1alpha1.TrashedResourceSpec{
			Data: string(podYAML),
		},
	}
	if err := c.Create(ctx, trashed); err != nil {
		log.Log.Error(err, "Erro ao criar TrashedResource")
	} else {
		log.Log.Info("TrashedResource criado com sucesso", "name", trashed.Name, "namespace", trashed.Namespace)
	}
}

// +kubebuilder:rbac:groups=mox.app.br,resources=trashedresources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mox.app.br,resources=trashedresources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mox.app.br,resources=trashedresources/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the trashedresources object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *TrashedResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Tenta buscar o Pod pelo nome e namespace do request
	var pod v1.Pod
	err := r.Get(ctx, req.NamespacedName, &pod)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Pod deletado", "namespace", req.Namespace, "name", req.Name)

			// Aqui você pode adicionar lógica extra se necessário
			return ctrl.Result{}, nil
		}
		// Outro erro ao buscar o Pod
		logger.Error(err, "Erro ao buscar Pod")
		return ctrl.Result{}, err
	}

	// Pod existe, lógica normal...

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TrashedResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&moxv1alpha1.TrashedResource{}).
		// TODO - Deixar isso dinamico com algum parametro dentro do Spec do TrashedResource, para o usuario escolher quais recursos quer monitorar
		Watches(&v1.Pod{}, &handler.EnqueueRequestForObject{}).
		Watches(&v1.Secret{}, &handler.EnqueueRequestForObject{}).
		Watches(&v1.ConfigMap{}, &handler.EnqueueRequestForObject{}).
		WithEventFilter(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				return true
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				getUpdatedOrDeletedManifest(r.Client, e.Object)
				return true
			},
		}).
		Named("trashedresources").
		Complete(r)
}
