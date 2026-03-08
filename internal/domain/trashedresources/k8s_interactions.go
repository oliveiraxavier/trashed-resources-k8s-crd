package trashedresources

import (
	"context"
	"fmt"
	"strings"
	moxv1alpha1 "trashed-resources/api/v1alpha1"

	utils "trashed-resources/internal/utils"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
type TRReconciler utils.TrashedResourceReconciler

func CreateOrUpdatedManifest(c client.Client, kubernetesObject client.Object, resourceReconciler *TRReconciler, actionType string) bool {
	ctx := context.Background()
	trInteractor := trashedResourceInteractor{client: c}
	objectYAML := utils.MakeBodyManifest(kubernetesObject)
	if objectYAML == nil {
		return false
	}
	dateTime := utils.Now().Format("20060102-150405")
	setName := fmt.Sprintf("trashed-%s-%s-%s-%s", actionType, strings.ToLower(kubernetesObject.GetObjectKind().GroupVersionKind().Kind), kubernetesObject.GetName(), dateTime)

	// Cria o TrashedResource
	trashed := &moxv1alpha1.TrashedResource{
		TypeMeta: metav1.TypeMeta{
			Kind:       "TrashedResource",
			APIVersion: "mox.app.br/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: setName,
			Name:         setName,
			Namespace:    kubernetesObject.GetNamespace(),
		},
		Spec: moxv1alpha1.TrashedResourceSpec{
			Data:      string(objectYAML),
			KeepUntil: utils.GetTimetoKeepFromConfigMap((*utils.TRReconciler)(resourceReconciler)),
		},
	}
	if err := trInteractor.Create(ctx, trashed); err != nil {
		logger.Error(err, "Error on create TrashedResource")
		return false
	}
	logger.Info("Success on create TrashedResource",
		"kubernetes_object", kubernetesObject.GetObjectKind().GroupVersionKind().Kind,
		"actionType", actionType,
		"name", trashed.Name,
		"namespace", trashed.Namespace,
	)
	return true
}

func GetToReconcile(ctx context.Context, c client.Client, name string, namespace string) (*moxv1alpha1.TrashedResource, error) {
	trInteractor := NewTrashedResourceInteractor(c)
	trashedResource, err := trInteractor.Get(ctx, name, namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return trashedResource, nil
}

func DeleteToReconcile(ctx context.Context, c client.Client, name string, namespace string) error {
	trInteractor := NewTrashedResourceInteractor(c)
	err := trInteractor.Delete(ctx, name, namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

// NewTrashedResourceInteractor cria um novo TrashedResourceInteractor.
func NewTrashedResourceInteractor(c client.Client) TrashedResourceInteractor {
	return &trashedResourceInteractor{client: c}
}

// Get recupera um TrashedResource pelo nome e namespace.
func (interactor *trashedResourceInteractor) Get(ctx context.Context, name, namespace string) (*moxv1alpha1.TrashedResource, error) {
	resource := &moxv1alpha1.TrashedResource{}
	err := interactor.client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, resource)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return resource, nil
}

// List recupera todos os TrashedResources em um determinado namespace.
// Se o namespace for uma string vazia, lista os recursos de todos os namespaces.
func (interactor *trashedResourceInteractor) List(ctx context.Context, namespace string) (*moxv1alpha1.TrashedResourceList, error) {
	list := &moxv1alpha1.TrashedResourceList{}
	opts := []client.ListOption{}

	if namespace != "" && namespace != "all" {
		opts = append(opts, client.InNamespace(namespace))
	}

	if err := interactor.client.List(ctx, list, opts...); err != nil {
		return nil, err
	}
	return list, nil
}

// Create cria um novo TrashedResource.
func (interactor *trashedResourceInteractor) Create(ctx context.Context, resource *moxv1alpha1.TrashedResource) error {
	return interactor.client.Create(ctx, resource)
}

// Update atualiza um TrashedResource existente.
func (interactor *trashedResourceInteractor) Update(ctx context.Context, resource *moxv1alpha1.TrashedResource) error {
	return interactor.client.Update(ctx, resource)
}

// Delete deleta um TrashedResource pelo nome e namespace.
func (interactor *trashedResourceInteractor) Delete(ctx context.Context, name, namespace string) error {
	resource := &moxv1alpha1.TrashedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return interactor.client.Delete(ctx, resource)
}
