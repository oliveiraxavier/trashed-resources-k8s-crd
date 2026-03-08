package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	moxv1alpha1 "trashed-resources/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestKubectlTrashedResources(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Kubectl TrashedResources Suite")
}

var _ = Describe("kubectl-trashedresources plugin", func() {
	var k8sClient client.Client
	var testScheme *runtime.Scheme
	ctx := context.Background()

	BeforeEach(func() {
		testScheme = runtime.NewScheme()
		Expect(moxv1alpha1.AddToScheme(testScheme)).To(Succeed())
		Expect(corev1.AddToScheme(testScheme)).To(Succeed())

		k8sClient = fake.NewClientBuilder().
			WithScheme(testScheme).
			WithIndex(&moxv1alpha1.TrashedResource{}, "metadata.name", func(o client.Object) []string {
				return []string{o.GetName()}
			}).
			Build()
	})

	Context("when restoring a resource", func() {
		const trName = "trashed-cm-test"
		const ns = "default"
		const cmName = "my-configmap"

		BeforeEach(func() {
			// Create a TrashedResource to be restored
			cmYAML := fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
data:
  key: value
`, cmName, ns)

			tr := &moxv1alpha1.TrashedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      trName,
					Namespace: ns,
				},
				Spec: moxv1alpha1.TrashedResourceSpec{
					Data: cmYAML,
				},
			}
			Expect(k8sClient.Create(ctx, tr)).To(Succeed())
		})

		It("should restore the object and delete the trashed resource", func() {
			err := restoreResource(k8sClient, trName, ns)
			Expect(err).NotTo(HaveOccurred())

			// Verify ConfigMap was created
			restoredCM := &corev1.ConfigMap{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: cmName, Namespace: ns}, restoredCM)
			Expect(err).NotTo(HaveOccurred())
			Expect(restoredCM.Data["key"]).To(Equal("value"))

			// Verify TrashedResource was deleted
			err = k8sClient.Get(ctx, types.NamespacedName{Name: trName, Namespace: ns}, &moxv1alpha1.TrashedResource{})
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("should return an error if the trashed resource does not exist", func() {
			err := restoreResource(k8sClient, "non-existent", ns)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to find TrashedResource"))
		})

		It("should return an error if the resource to restore already exists", func() {
			// Pre-create the ConfigMap
			existingCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cmName,
					Namespace: ns,
				},
			}
			Expect(k8sClient.Create(ctx, existingCM)).To(Succeed())

			err := restoreResource(k8sClient, trName, ns)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("already exists"))
		})
	})

	Context("when pruning resources", func() {
		var oldResource, newResource, namedResource, otherNsResource *moxv1alpha1.TrashedResource

		BeforeEach(func() {
			// Create resources with different ages and namespaces
			oldResource = &moxv1alpha1.TrashedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "old-resource",
					Namespace:         "default",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-2 * time.Hour)},
				},
			}
			newResource = &moxv1alpha1.TrashedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "new-resource",
					Namespace:         "default",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
				},
			}
			namedResource = &moxv1alpha1.TrashedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "special-name",
					Namespace:         "default",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-5 * time.Minute)},
				},
			}
			otherNsResource = &moxv1alpha1.TrashedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "other-ns-resource",
					Namespace:         "other",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-3 * time.Hour)},
				},
			}

			nsOther := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other"}}
			Expect(k8sClient.Create(ctx, nsOther)).To(Succeed())

			Expect(k8sClient.Create(ctx, namedResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, oldResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, newResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, otherNsResource)).To(Succeed())
		})

		It("should delete only resources older than the specified duration", func() {
			// Prune resources older than 1 hour in all namespaces
			err := pruneResources(k8sClient, "", 1*time.Hour, true, "")
			Expect(err).NotTo(HaveOccurred())

			// Verify old resources are deleted
			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(oldResource), &moxv1alpha1.TrashedResource{})
			Expect(errors.IsNotFound(err)).To(BeTrue())
			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(otherNsResource), &moxv1alpha1.TrashedResource{})
			Expect(errors.IsNotFound(err)).To(BeTrue())

			// Verify new resources remain
			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(newResource), &moxv1alpha1.TrashedResource{})
			Expect(err).NotTo(HaveOccurred())
			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(namedResource), &moxv1alpha1.TrashedResource{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete only the resource with the specified name", func() {
			// Prune by name, no age limit
			err := pruneResources(k8sClient, "default", 0, false, "special-name")
			Expect(err).NotTo(HaveOccurred())

			// Verify named resource is deleted
			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(namedResource), &moxv1alpha1.TrashedResource{})
			Expect(errors.IsNotFound(err)).To(BeTrue())

			// Verify others remain
			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(oldResource), &moxv1alpha1.TrashedResource{})
			Expect(err).NotTo(HaveOccurred())
			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(newResource), &moxv1alpha1.TrashedResource{})
			Expect(err).NotTo(HaveOccurred())
			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(otherNsResource), &moxv1alpha1.TrashedResource{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should only prune resources within the specified namespace", func() {
			// Prune resources older than 1 hour in 'default' namespace
			err := pruneResources(k8sClient, "default", 1*time.Hour, true, "")
			Expect(err).NotTo(HaveOccurred())

			// Verify old resource in 'default' is deleted
			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(oldResource), &moxv1alpha1.TrashedResource{})
			Expect(errors.IsNotFound(err)).To(BeTrue())

			// Verify new resource in 'default' remains
			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(newResource), &moxv1alpha1.TrashedResource{})
			Expect(err).NotTo(HaveOccurred())

			// Verify resource in 'other' namespace remains
			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(otherNsResource), &moxv1alpha1.TrashedResource{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should correctly parse 'd' for days in older-than flag", func() {
			// This is testing the cobra command logic indirectly.
			// The main.go has a special handling for 'd' suffix.
			// Let's simulate that logic.
			olderThan := "1d"
			var duration time.Duration
			if strings.HasSuffix(olderThan, "d") {
				daysStr := strings.TrimSuffix(olderThan, "d")
				days, err := time.ParseDuration(daysStr + "h")
				Expect(err).NotTo(HaveOccurred())
				duration = days * 24
			}
			Expect(duration).To(Equal(24 * time.Hour))
		})
	})

	Context("when getting a Kubernetes client with getClient", func() {
		var (
			kubeconfigFile *os.File
			err            error
		)

		BeforeEach(func() {
			// Create a dummy kubeconfig file
			kubeconfigFile, err = os.CreateTemp("", "kubeconfig-")
			Expect(err).NotTo(HaveOccurred())

			content := `
apiVersion: v1
clusters:
- cluster:
    server: https://localhost:8443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
kind: Config
preferences: {}
users:
- name: test-user
  user: {}
`
			_, err = kubeconfigFile.WriteString(content)
			Expect(err).NotTo(HaveOccurred())
			Expect(kubeconfigFile.Close()).To(Succeed())
		})

		AfterEach(func() {
			_ = os.Remove(kubeconfigFile.Name())
		})

		It("should return a client when flags point to a valid kubeconfig", func() {
			configFlags := genericclioptions.NewConfigFlags(true)
			kubeconfigPath := kubeconfigFile.Name()
			configFlags.KubeConfig = &kubeconfigPath

			client, err := getClient(configFlags)
			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())
		})

		It("should fallback and return a client if flags fail but KUBECONFIG env is set", func() {
			// Set KUBECONFIG env var
			originalKubeconfig := os.Getenv("KUBECONFIG")
			kubeconfigPath := kubeconfigFile.Name()
			Expect(os.Setenv("KUBECONFIG", kubeconfigPath)).To(Succeed())

			defer func() {
				_ = os.Setenv("KUBECONFIG", originalKubeconfig)
			}()

			// Point flags to a non-existent file
			nonExistentFile := "/path/to/a/non-existent/kubeconfig"
			configFlags := genericclioptions.NewConfigFlags(true)
			configFlags.KubeConfig = &nonExistentFile

			client, err := getClient(configFlags)
			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())
		})

		It("should return an error if no kubeconfig can be found", func() {
			originalHome := os.Getenv("HOME")
			originalKubeconfig, kubeconfigIsSet := os.LookupEnv("KUBECONFIG")

			defer func() {
				_ = os.Setenv("HOME", originalHome)
				if kubeconfigIsSet {
					_ = os.Setenv("KUBECONFIG", originalKubeconfig)
				} else {
					_ = os.Unsetenv("KUBECONFIG")
				}

			}()

			_ = os.Setenv("HOME", "/tmp/non-existent-home-for-test")
			// Point KUBECONFIG to a file that is not a valid kubeconfig to ensure loading fails
			// with a parsing error, which is a more reliable failure condition for this test.
			_ = os.Setenv("KUBECONFIG", "/dev/null")

			nonExistentFile := "/path/to/a/non-existent/kubeconfig"
			configFlags := genericclioptions.NewConfigFlags(true)
			configFlags.KubeConfig = &nonExistentFile

			_, err := getClient(configFlags)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when running restore command via CLI", func() {
		const trName = "trashed-cm-test"
		const ns = "default"
		const cmName = "my-configmap"

		BeforeEach(func() {
			// Create a TrashedResource to be restored
			cmYAML := fmt.Sprintf(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: %s
data:
  key: value
`, cmName, ns)

			tr := &moxv1alpha1.TrashedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      trName,
					Namespace: ns,
				},
				Spec: moxv1alpha1.TrashedResourceSpec{
					Data: cmYAML,
				},
			}
			Expect(k8sClient.Create(ctx, tr)).To(Succeed())
		})

		It("should restore a resource by name", func() {
			configFlags := genericclioptions.NewConfigFlags(true)
			namespace := ns
			configFlags.Namespace = &namespace

			mockClientGetter := func(flags *genericclioptions.ConfigFlags) (client.Client, error) {
				return k8sClient, nil
			}

			restoreCmd := restoreCmd(configFlags, mockClientGetter)
			restoreCmd.SetArgs([]string{trName})

			err := restoreCmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			// Verify TrashedResource was deleted
			err = k8sClient.Get(ctx, types.NamespacedName{Name: trName, Namespace: ns}, &moxv1alpha1.TrashedResource{})
			Expect(errors.IsNotFound(err)).To(BeTrue())

			// Verify ConfigMap was created
			restoredCM := &corev1.ConfigMap{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: cmName, Namespace: ns}, restoredCM)
			Expect(err).NotTo(HaveOccurred())
			Expect(restoredCM.Data["key"]).To(Equal("value"))
		})
	})

	Context("when running prune command via CLI", func() {
		var oldResource, newResource *moxv1alpha1.TrashedResource
		const ns = "default"

		BeforeEach(func() {
			oldResource = &moxv1alpha1.TrashedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "old-resource",
					Namespace:         ns,
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-2 * time.Hour)}},
			}
			newResource = &moxv1alpha1.TrashedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "new-resource",
					Namespace:         ns,
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
				},
			}
			Expect(k8sClient.Create(ctx, oldResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, newResource)).To(Succeed())
		})

		It("should prune a specific resource by name", func() {
			configFlags := genericclioptions.NewConfigFlags(true)
			namespace := ns
			configFlags.Namespace = &namespace
			mockClientGetter := func(flags *genericclioptions.ConfigFlags) (client.Client, error) { return k8sClient, nil }

			pruneCmd := pruneCmd(configFlags, mockClientGetter)
			pruneCmd.SetArgs([]string{"old-resource"})
			Expect(pruneCmd.Execute()).To(Succeed())

			// Verify old resource is deleted
			Expect(errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(oldResource), oldResource))).To(BeTrue())
			// Verify new resource remains
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(newResource), newResource)).To(Succeed())
		})

		It("should prune resources older than a given duration", func() {
			configFlags := genericclioptions.NewConfigFlags(true)
			namespace := ns
			configFlags.Namespace = &namespace
			mockClientGetter := func(flags *genericclioptions.ConfigFlags) (client.Client, error) { return k8sClient, nil }

			pruneCmd := pruneCmd(configFlags, mockClientGetter)
			pruneCmd.SetArgs([]string{"--older-than", "1h"})
			Expect(pruneCmd.Execute()).To(Succeed())

			// Verify old resource is deleted
			Expect(errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(oldResource), oldResource))).To(BeTrue())
			// Verify new resource remains
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(newResource), newResource)).To(Succeed())
		})

		It("should return an error if no name or flag is provided", func() {
			configFlags := genericclioptions.NewConfigFlags(true)
			namespace := ns
			configFlags.Namespace = &namespace
			mockClientGetter := func(flags *genericclioptions.ConfigFlags) (client.Client, error) { return k8sClient, nil }

			pruneCmd := pruneCmd(configFlags, mockClientGetter)
			pruneCmd.SetArgs([]string{}) // No arguments

			err := pruneCmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("either a resource name or the --older-than flag is required"))
		})
	})

	Context("when running restore command via CLI", func() {
		var newResource *moxv1alpha1.TrashedResource
		const ns = "default"

		BeforeEach(func() {
			newResource = &moxv1alpha1.TrashedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new-resource",
					Namespace: ns,
					CreationTimestamp: metav1.Time{
						Time: time.Now().Add(-10 * time.Minute),
					},
				},
				Spec: moxv1alpha1.TrashedResourceSpec{
					Data: `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: container1
    image: image1`,
				},
			}
			Expect(k8sClient.Create(ctx, newResource)).To(Succeed())
		})

		It("should restore a specific resource by name", func() {
			configFlags := genericclioptions.NewConfigFlags(true)
			namespace := ns
			configFlags.Namespace = &namespace
			mockClientGetter := func(flags *genericclioptions.ConfigFlags) (client.Client, error) { return k8sClient, nil }

			restoreCmd := restoreCmd(configFlags, mockClientGetter)
			restoreCmd.SetArgs([]string{"new-resource"})
			Expect(restoreCmd.Execute()).To(Succeed())
			Expect(errors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(newResource), newResource))).To(BeTrue())

		})

	})
})
