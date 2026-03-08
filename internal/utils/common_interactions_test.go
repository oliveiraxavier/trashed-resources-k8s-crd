package utils

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMakeBodyManifest(t *testing.T) {
	g := NewWithT(t)

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
		},
	}

	manifest := MakeBodyManifest(deployment)
	g.Expect(manifest).NotTo(BeNil())
	g.Expect(string(manifest)).To(ContainSubstring("kind: Deployment"))
	g.Expect(string(manifest)).To(ContainSubstring("name: test-deployment"))
}

func TestMakeBodyManifest_RemovesManagedFields(t *testing.T) {
	g := NewWithT(t)

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			ManagedFields: []metav1.ManagedFieldsEntry{
				{
					Manager:    "kubectl",
					Operation:  "Update",
					APIVersion: "v1",
				},
			},
		},
	}

	manifest := MakeBodyManifest(pod)
	g.Expect(manifest).NotTo(BeNil())

	manifestStr := string(manifest)
	g.Expect(manifestStr).To(ContainSubstring("kind: Pod"))
	g.Expect(manifestStr).NotTo(ContainSubstring("managedFields"))
}

func TestMakeBodyManifest_InfersAPIVersion(t *testing.T) {
	g := NewWithT(t)

	// Secret with Kind set but APIVersion missing to test inference logic
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind: "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
	}

	manifest := MakeBodyManifest(secret)
	g.Expect(manifest).NotTo(BeNil())

	manifestStr := string(manifest)
	g.Expect(manifestStr).To(ContainSubstring("kind: Secret"))
	g.Expect(manifestStr).To(ContainSubstring("apiVersion: v1"))
}

func TestGetTimetoKeepFromConfigMap(t *testing.T) {
	g := NewWithT(t)

	reconciler := &TRReconciler{
		MinutesToKeep: "10",
		HoursToKeep:   "1",
		DaysToKeep:    "0",
	}

	keepUntil := GetTimetoKeepFromConfigMap(reconciler)
	g.Expect(keepUntil).NotTo(BeEmpty())

	// Parse the result to verify it's a valid date
	parsedTime, err := time.Parse(time.RFC3339, keepUntil)
	g.Expect(err).NotTo(HaveOccurred())

	// It should be in the future (1h 10m from now)
	g.Expect(parsedTime.After(time.Now())).To(BeTrue())
}
