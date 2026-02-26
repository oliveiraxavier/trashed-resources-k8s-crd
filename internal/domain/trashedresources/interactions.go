package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	moxv1alpha1 "trashed-resources/api/v1alpha1"
	utils "trashed-resources/internal/utils"

	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var logger = log.Log

// TrashedResourceInteractor define a interface para interagir com objetos TrashedResource.
type TrashedResourceInteractor interface {
	Get(ctx context.Context, name, namespace string) (*moxv1alpha1.TrashedResource, error)
	List(ctx context.Context, namespace string) (*moxv1alpha1.TrashedResourceList, error)
	Create(ctx context.Context, resource *moxv1alpha1.TrashedResource) error
	Update(ctx context.Context, resource *moxv1alpha1.TrashedResource) error
	Delete(ctx context.Context, name, namespace string) error
}

// trashedResourceInteractor implementa a interface TrashedResourceInteractor.
type trashedResourceInteractor struct {
	client client.Client
}

func makeBodyManifest(kubernetesObj client.Object) []byte {
	objectJSON, err := json.Marshal(kubernetesObj)
	if err != nil {
		logger.Error(err, "Error serializing object to JSON")
		return nil
	}
	var objectMap map[string]interface{}
	if err := json.Unmarshal(objectJSON, &objectMap); err != nil {
		logger.Error(err, "Error deserializing object JSON")
		return nil
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
	knownGVKs := utils.GetKnownKindsToWatch()
	if apiVersion == "" && kind != "" {
		if rgvk, ok := knownGVKs[strings.ToLower(kind)]; ok {
			if rgvk.Group != "" {
				apiVersion = rgvk.Group + "/" + rgvk.Version
			} else {
				apiVersion = rgvk.Version
			}
		}
	}
	if _, ok := objectMap["kind"]; !ok && kind != "" {
		objectMap["kind"] = kind
	}
	if _, ok := objectMap["apiVersion"]; !ok && apiVersion != "" {
		objectMap["apiVersion"] = apiVersion
	}

	// Remove managedFields from metadata when present to simplify the YAML
	if md, ok := objectMap["metadata"].(map[string]interface{}); ok {
		delete(md, "managedFields")
	}

	objectYAML, err := yaml.Marshal(objectMap)
	if err != nil {
		logger.Error(err, "Error serializing object to YAML")
		return nil
	}

	return objectYAML
}

func getTimetoKeepFromConfigMap(configMapData v1.ConfigMap) string {
	minutesToKeep := int64(60) // default value
	hoursToKeep := int64(0)    // default value
	dateNow := utils.Now()
	// .AddHours(24).ToString()
	if val, ok := configMapData.Data["minutesToKeep"]; ok {
		if mtk, err := strconv.Atoi(val); err == nil {
			minutesToKeep = int64(mtk)
		} else {
			logger.Error(err, "Invalid value for minutesToKeep in ConfigMap, using default", "value", val)
		}

	}

	if val, ok := configMapData.Data["hoursToKeep"]; ok {
		if htk, err := strconv.Atoi(val); err == nil {
			hoursToKeep = int64(htk)
		} else {
			logger.Error(err, "Invalid value for hoursToKeep in ConfigMap, using default", "value", val)
		}
	}
	return dateNow.AddMinutes(minutesToKeep).AddHours(hoursToKeep).ToString()
}
func CreateOrUpdatedManifest(c client.Client, kubernetesObj client.Object, configMapData v1.ConfigMap, action_type string) bool {
	ctx := context.Background()
	trInteractor := NewTrashedResourceInteractor(c)
	if action_type == "create" {
		objectYAML := makeBodyManifest(kubernetesObj)
		if objectYAML == nil {
			return false
		}
		// Cria o TrashedResource
		trashed := &moxv1alpha1.TrashedResource{
			TypeMeta: metav1.TypeMeta{
				Kind:       "TrashedResource",
				APIVersion: "mox.app.br/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: fmt.Sprintf("trashed-%s-%s-%s-", action_type, strings.ToLower(kubernetesObj.GetObjectKind().GroupVersionKind().Kind), kubernetesObj.GetName()),
				Namespace:    kubernetesObj.GetNamespace(),
			},
			Spec: moxv1alpha1.TrashedResourceSpec{
				Data:      string(objectYAML),
				KeepUntil: getTimetoKeepFromConfigMap(configMapData),
				// KeepUntil: utils.Now().AddMinutes(1).ToString(), // Para testes rápidos
			},
		}
		if err := trInteractor.Create(ctx, trashed); err != nil {
			logger.Error(err, "Error on  "+action_type+" TrashedResource")
			return false
		}
		logger.Info("Success on "+action_type+" TrashedResource", "name", trashed.Name, "namespace", trashed.Namespace)
	}
	return true
}

// NewTrashedResourceInteractor cria um novo TrashedResourceInteractor.
func NewTrashedResourceInteractor(c client.Client) TrashedResourceInteractor {
	return &trashedResourceInteractor{client: c}
}

// Get recupera um TrashedResource pelo nome e namespace.
func (i *trashedResourceInteractor) Get(ctx context.Context, name, namespace string) (*moxv1alpha1.TrashedResource, error) {
	resource := &moxv1alpha1.TrashedResource{}
	err := i.client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, resource)
	if err != nil {
		return nil, err
	}
	return resource, nil
}

// List recupera todos os TrashedResources em um determinado namespace.
// Se o namespace for uma string vazia, lista os recursos de todos os namespaces.
func (i *trashedResourceInteractor) List(ctx context.Context, namespace string) (*moxv1alpha1.TrashedResourceList, error) {
	list := &moxv1alpha1.TrashedResourceList{}
	opts := []client.ListOption{}
	if namespace != "" {
		opts = append(opts, client.InNamespace(namespace))
	}

	if err := i.client.List(ctx, list, opts...); err != nil {
		return nil, err
	}
	return list, nil
}

// Create cria um novo TrashedResource.
func (i *trashedResourceInteractor) Create(ctx context.Context, resource *moxv1alpha1.TrashedResource) error {
	return i.client.Create(ctx, resource)
}

// Update atualiza um TrashedResource existente.
func (i *trashedResourceInteractor) Update(ctx context.Context, resource *moxv1alpha1.TrashedResource) error {
	return i.client.Update(ctx, resource)
}

// Delete deleta um TrashedResource pelo nome e namespace.
func (i *trashedResourceInteractor) Delete(ctx context.Context, name, namespace string) error {
	resource := &moxv1alpha1.TrashedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return i.client.Delete(ctx, resource)
}
