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
	"encoding/json"
	"fmt"
	"strings"
	moxv1alpha1 "trashed-resources/api/v1alpha1"
	utils "trashed-resources/internal/utils"

	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		log.Log.Error(err, "Error serializing object to JSON")
		return
	}
	var podMap map[string]interface{}
	if err := json.Unmarshal(podJSON, &podMap); err != nil {
		log.Log.Error(err, "Error deserializing object JSON")
		return
	}
	// Ensure apiVersion and kind are present in the serialized object
	gvk := kubernetesObj.GetObjectKind().GroupVersionKind()
	kind := gvk.Kind
	apiVersion := ""
	if gvk.Version != "" {
		if gvk.Group != "" {
			apiVersion = gvk.Group + "/" + gvk.Version
		} else {
			apiVersion = gvk.Version
		}
	}
	// Fallback: try to infer from knownGVKs map using kind lowercased
	if apiVersion == "" && kind != "" {
		if rgvk, ok := knownGVKs[strings.ToLower(kind)]; ok {
			if rgvk.Group != "" {
				apiVersion = rgvk.Group + "/" + rgvk.Version
			} else {
				apiVersion = rgvk.Version
			}
		}
	}
	if _, ok := podMap["kind"]; !ok && kind != "" {
		podMap["kind"] = kind
	}
	if _, ok := podMap["apiVersion"]; !ok && apiVersion != "" {
		podMap["apiVersion"] = apiVersion
	}

	// Remove managedFields from metadata when present to simplify the YAML
	if md, ok := podMap["metadata"].(map[string]interface{}); ok {
		delete(md, "managedFields")
	}
	podYAML, err := yaml.Marshal(podMap)
	if err != nil {
		log.Log.Error(err, "Error serializing object to YAML")
		return
	}
	log.Log.Info("Deleted object's manifest:", "yaml", kubernetesObj.GetName())

	// Cria o TrashedResource
	trashed := &moxv1alpha1.TrashedResource{
		TypeMeta: metav1.TypeMeta{
			Kind:       "TrashedResource",
			APIVersion: "mox.app.br/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("trashed-%s-%s-", strings.ToLower(kubernetesObj.GetObjectKind().GroupVersionKind().Kind), kubernetesObj.GetName()),
			Namespace:    kubernetesObj.GetNamespace(),
		},
		Spec: moxv1alpha1.TrashedResourceSpec{
			Data:      string(podYAML),
			KeepUntil: utils.Now().AddHours(24).ToString(), // TODO definir o tempo de retenção de forma configurável via configmap por exemplo
			// KeepUntil: utils.Now().AddMinutes(1).ToString(), // Para testes rápidos
		},
	}
	if err := c.Create(ctx, trashed); err != nil {
		log.Log.Error(err, "Error creating TrashedResource")
		return
	}
	log.Log.Info("TrashedResource created successfully", "name", trashed.Name, "namespace", trashed.Namespace)

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

	rawKinds := strings.Split(cm.Data["kindsTobserve"], ";")
	rawKinds = strings.Fields(strings.Join(rawKinds, " ")) // Remove espaços extras
	logger.Info("ConfigMap "+cmName+" loaded. Kinds found to watch", "kinds", rawKinds)
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&moxv1alpha1.TrashedResource{}).
		WithEventFilter(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool { return false },
			DeleteFunc: func(e event.DeleteEvent) bool {
				if e.Object.GetObjectKind().GroupVersionKind().Kind == "" {
					return false
				}
				createUpdatedOrDeletedManifest(mgr.GetClient(), e.Object)
				return true
			},
		}).
		Named("trashedresources")

	// Para cada Kind, adiciona um watch dinâmico
	for _, k := range rawKinds {
		kind := strings.TrimSpace(k)
		if kind == "" {
			continue
		}

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
	builder = builder.Watches(&moxv1alpha1.TrashedResource{}, &handler.EnqueueRequestForObject{})

	return builder.Complete(r)
}
