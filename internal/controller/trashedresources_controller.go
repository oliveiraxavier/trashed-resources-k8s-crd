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

	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var logger = log.Log

// TrashedResourceReconciler reconciles a trashedresources object
type TrashedResourceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// logPodManifestDelete loga o manifesto YAML do Pod deletado
func createUpdatedOrDeletedManifest(c client.Client, kubernetesObj client.Object) {
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
			GenerateName: fmt.Sprintf("trashed-%s-%s-", strings.ToLower(kubernetesObj.GetObjectKind().GroupVersionKind().Kind), kubernetesObj.GetName()),
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

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TrashedResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// É necessário definir o GVK para que o controller saiba qual recurso monitorar via Unstructured
	setup := ctrl.NewControllerManagedBy(mgr).
		For(&moxv1alpha1.TrashedResource{}).
		WithEventFilter(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				return true
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return true
			},
		}).
		Named("trashedresources")

	items, err := getToWatch(r.Client, context.Background())
	if err != nil {
		return err
	}
	for _, item := range items {
		logger.Info("Adicionando watch para tipo:", "kind", item)
		// u := &unstructured.Unstructured{}
		// u.SetGroupVersionKind(item)
		// setup.Watches(u, &handler.EnqueueRequestForObject{})
		//Watches(&item{}, &handler.EnqueueRequestForObject{}).
		// Watches(&appsv1.Deployment{}, &handler.EnqueueRequestForObject{}).
		// Watches(&v1.Secret{}, &handler.EnqueueRequestForObject{}).
		// Watches(&v1.ConfigMap{}, &handler.EnqueueRequestForObject{}).
	}
	return setup.Complete(r)
}

func getToWatch(c client.Client, ctx context.Context) ([]string, error) {
	// Lê o ConfigMap para obter os tipos permitidos
	cm := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: "system", Name: "trashedresources-config"}, cm); err != nil {
		logger.Error(err, "Erro ao ler ConfigMap")
		return nil, err
	}

	var result []string
	if val, ok := cm.Data["toWatch"]; ok {
		for _, entry := range strings.Split(val, ";") {
			kind := strings.TrimSpace(entry)
			result = append(result, kind)
		}
	}
	return result, nil
}
