package trashedresources

import (
	"context"
	"testing"

	moxv1alpha1 "trashed-resources/api/v1alpha1"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewTrashedResourceInteractor(t *testing.T) {
	g := NewWithT(t)
	c := fake.NewClientBuilder().Build()
	interactor := NewTrashedResourceInteractor(c)
	g.Expect(interactor).NotTo(BeNil())
}

func TestGetToReconcile(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	_ = moxv1alpha1.AddToScheme(scheme)

	tr := &moxv1alpha1.TrashedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tr",
			Namespace: "default",
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tr).Build()
	ctx := context.Background()

	// Test existing
	res, err := GetToReconcile(ctx, c, "test-tr", "default")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res).NotTo(BeNil())
	g.Expect(res.Name).To(Equal("test-tr"))

	// Test not found
	res, err = GetToReconcile(ctx, c, "non-existent", "default")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res).To(BeNil())
}

func TestDeleteToReconcile(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	_ = moxv1alpha1.AddToScheme(scheme)

	tr := &moxv1alpha1.TrashedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-tr",
			Namespace: "default",
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tr).Build()
	ctx := context.Background()

	// Test delete existing
	err := DeleteToReconcile(ctx, c, "test-tr", "default")
	g.Expect(err).NotTo(HaveOccurred())

	// Verify deletion
	err = c.Get(ctx, types.NamespacedName{Name: "test-tr", Namespace: "default"}, &moxv1alpha1.TrashedResource{})
	g.Expect(err).To(HaveOccurred())

	// Test delete non-existent
	err = DeleteToReconcile(ctx, c, "non-existent", "default")
	g.Expect(err).NotTo(HaveOccurred())
}

func TestCreateOrUpdatedManifest(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	_ = moxv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
	}

	reconciler := &TRReconciler{
		MinutesToKeep: "60",
	}

	success := CreateOrUpdatedManifest(c, pod, reconciler, "deleted")
	g.Expect(success).To(BeTrue())

	list := &moxv1alpha1.TrashedResourceList{}
	err := c.List(context.Background(), list)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(list.Items).To(HaveLen(1))
	g.Expect(list.Items[0].Namespace).To(Equal("default"))
	g.Expect(list.Items[0].Name).To(ContainSubstring("trashed-deleted-pod-test-pod-"))
}
