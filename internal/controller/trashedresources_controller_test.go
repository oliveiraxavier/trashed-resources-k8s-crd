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
	"bytes"
	"context"
	"regexp"
	"time"
	moxv1alpha1 "trashed-resources/api/v1alpha1"
	utils "trashed-resources/internal/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("TrashedResource Controller", func() {
	Context("When reconciling a resource", func() {

		const resourceName = "test-resource"
		var k8sClient client.Client

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			k8sClient = fake.NewClientBuilder().
				WithStatusSubresource(&moxv1alpha1.TrashedResource{}).
				Build()
			By("creating the custom resource for the Kind trashedresources")
			deployment := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: func(i int32) *int32 { return &i }(1),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "test"},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx:latest",
								},
							},
						},
					},
				},
			}

			objectYAML := utils.MakeBodyManifest(deployment)
			Expect(objectYAML).NotTo(BeNil())

			resource := &moxv1alpha1.TrashedResource{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err != nil && errors.IsNotFound(err) {
				resource := &moxv1alpha1.TrashedResource{
					TypeMeta: metav1.TypeMeta{
						Kind:       "TrashedResource",
						APIVersion: "mox.app.br/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: resourceName,
						Name:         resourceName,
						Namespace:    "default",
					},
					Spec: moxv1alpha1.TrashedResourceSpec{
						Data: string(objectYAML),
						KeepUntil: utils.GetTimetoKeepFromConfigMap(&utils.TRReconciler{
							MinutesToKeep: "60",
							HoursToKeep:   "0",
							DaysToKeep:    "0",
						}),
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &moxv1alpha1.TrashedResource{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance trashedresources")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &TrashedResourceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when setting up watches with appendKindsToWatch", func() {
		var (
			builder   *ctrl.Builder
			logBuffer bytes.Buffer
		)

		BeforeEach(func() {
			logBuffer.Reset()

			originalLogger := logger

			testLogger := zap.New(zap.WriteTo(&logBuffer), zap.UseDevMode(true))
			logger = testLogger
			log.SetLogger(testLogger)

			DeferCleanup(func() {
				logger = originalLogger
				log.SetLogger(testLogger)
			})

			// A manager is needed to create a builder.
			// We can use the cfg from the test suite.
			mgr, err := ctrl.NewManager(cfg, ctrl.Options{
				Scheme: k8sClient.Scheme(),
				// Disable metrics to avoid port conflicts
				Metrics: server.Options{BindAddress: "0"},
			})
			Expect(err).NotTo(HaveOccurred())
			builder = ctrl.NewControllerManagedBy(mgr)
		})

		It("should add watches for valid kinds and ignore invalid ones", func() {
			cm := &corev1.ConfigMap{
				Data: map[string]string{
					"kindsToObserve": "Deployment; Pod",
				},
			}

			appendKindsToWatch(builder, *cm)

			logs := logBuffer.String()

			Expect(logs).To(MatchRegexp(`Watching kind\s+\{"kind": "Deployment"\}`))
			Expect(logs).To(MatchRegexp(`Kind not explicitly mapped; ignoring.\s+\{"kind": "Pod"\}`))
		})

		It("should handle empty or whitespace-only kind strings", func() {
			cm := &corev1.ConfigMap{
				Data: map[string]string{
					"kindsToObserve": "",
				},
			}

			appendKindsToWatch(builder, *cm)

			logs := logBuffer.String()
			Expect(logs).To(BeEmpty())
		})

		It("should handle kinds with leading/trailing whitespace", func() {
			cm := &corev1.ConfigMap{
				Data: map[string]string{
					"kindsToObserve": "  Deployment  ;  Pod ",
				},
			}

			appendKindsToWatch(builder, *cm)

			logs := logBuffer.String()

			Expect(logs).To(MatchRegexp(`Watching kind\s+\{"kind": "Deployment"\}`))
			Expect(logs).To(MatchRegexp(`Kind not explicitly mapped; ignoring.\s+\{"kind": "Pod"\}`))
			expectedLog := "Kinds mapped are: " + regexp.QuoteMeta(utils.KnownGVKsAsString())
			Expect(logs).To(MatchRegexp(expectedLog))

		})

		It("should not add any watches if kindsToObserve is not present in known kinds", func() {
			cm := &corev1.ConfigMap{
				Data: map[string]string{
					"kindsToObserve": "someValue",
				},
			}

			appendKindsToWatch(builder, *cm)

			logs := logBuffer.String()
			// Expect(logs).To(ContainSubstring("Kind not explicitly mapped; ignoring."))
			Expect(logs).To(MatchRegexp(`Kind not explicitly mapped; ignoring.\s+\{"kind": "someValue"\}`))
			expectedLog := "Kinds mapped are: " + regexp.QuoteMeta(utils.KnownGVKsAsString())
			Expect(logs).To(MatchRegexp(expectedLog))
		})
	})

	Context("When handling expiration logic", func() {
		ctx := context.Background()
		var k8sClient client.Client
		var reconciler *TrashedResourceReconciler

		BeforeEach(func() {
			k8sClient = fake.NewClientBuilder().
				WithStatusSubresource(&moxv1alpha1.TrashedResource{}).
				Build()

			reconciler = &TrashedResourceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
		})

		It("should delete the resource if it is expired", func() {
			expiredName := "expired-resource"
			// Create a resource with KeepUntil in the past
			expiredTR := &moxv1alpha1.TrashedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      expiredName,
					Namespace: "default",
				},
				Spec: moxv1alpha1.TrashedResourceSpec{
					Data:      "some-data",
					KeepUntil: time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
				},
			}
			Expect(k8sClient.Create(ctx, expiredTR)).To(Succeed())

			// Reconcile
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: expiredName, Namespace: "default"},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify deletion
			err = k8sClient.Get(ctx, types.NamespacedName{Name: expiredName, Namespace: "default"}, &moxv1alpha1.TrashedResource{})
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("should requeue if the resource is not expired", func() {
			futureName := "future-resource"
			// Create a resource with KeepUntil in the future
			futureTR := &moxv1alpha1.TrashedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      futureName,
					Namespace: "default",
				},
				Spec: moxv1alpha1.TrashedResourceSpec{
					Data:      "some-data",
					KeepUntil: time.Now().Add(1 * time.Hour).Format(time.RFC3339),
				},
			}
			Expect(k8sClient.Create(ctx, futureTR)).To(Succeed())

			// Reconcile
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: futureName, Namespace: "default"},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify Requeue
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))
			Expect(result.RequeueAfter).To(BeNumerically("<=", 1*time.Hour+time.Minute))
		})

		It("should ignore if resource is not found", func() {
			// Reconcile a non-existent resource
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "ghost", Namespace: "default"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})
	})

	Context("When setting up the controller with Manager", func() {
		var (
			mgr        ctrl.Manager
			reconciler *TrashedResourceReconciler
			cm         *corev1.ConfigMap
			ns         *corev1.Namespace
		)

		BeforeEach(func() {
			// Ensure 'system' namespace exists for the ConfigMap
			ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "trashed-resources-system",
				},
			}
			// Create namespace if it doesn't exist
			if err := k8sClient.Create(ctx, ns); err != nil {
				Expect(errors.IsAlreadyExists(err)).To(BeTrue())
			}

			// Create the ConfigMap
			cm = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "trashedresources-config",
					Namespace: "trashed-resources-system",
				},
				Data: map[string]string{
					"kindsToObserve":     "Deployment;Secret",
					"actionsToObserve":   "delete",
					"namespacesToIgnore": "kube-system",
					"minutesToKeep":      "30",
					"hoursToKeep":        "1",
					"daysToKeep":         "0",
				},
			}

			_ = k8sClient.Delete(ctx, cm)
			Expect(k8sClient.Create(ctx, cm)).To(Succeed())

			var err error
			mgr, err = ctrl.NewManager(cfg, ctrl.Options{
				Scheme:  k8sClient.Scheme(),
				Metrics: server.Options{BindAddress: "0"},
			})
			Expect(err).NotTo(HaveOccurred())

			reconciler = &TrashedResourceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, cm)
		})

		It("should successfully setup with manager and load configuration", func() {
			_ = reconciler.SetupWithManager(mgr)
			// Verify configuration loaded into reconciler
			Expect(reconciler.KindsToWatch).To(ConsistOf("Deployment", "Secret"))
			Expect(reconciler.ActionsToWatch).To(ConsistOf("delete"))
			Expect(reconciler.NamespacesToIgnore).To(ConsistOf("kube-system"))
			Expect(reconciler.MinutesToKeep).To(Equal("30"))
			Expect(reconciler.HoursToKeep).To(Equal("1"))
		})

		It("should use default configuration when ConfigMap is missing", func() {
			Expect(k8sClient.Delete(ctx, cm)).To(Succeed())
			_ = reconciler.SetupWithManager(mgr)

			// When configmap not set kindsToObserve then use as defined in utils.GetAllConfigsFromConfigMap
			Expect(reconciler.KindsToWatch).To(ContainElements("Deployment", "Secret", "ConfigMap"))
			Expect(reconciler.ActionsToWatch).To(ContainElement("delete"))
		})
	})

	Context("When handling Update events", func() {
		var (
			reconciler *TrashedResourceReconciler
			fakeClient client.Client
		)

		BeforeEach(func() {
			fakeClient = fake.NewClientBuilder().
				WithScheme(k8sClient.Scheme()).
				WithStatusSubresource(&moxv1alpha1.TrashedResource{}).
				Build()

			reconciler = &TrashedResourceReconciler{
				Client:             fakeClient,
				Scheme:             k8sClient.Scheme(),
				ActionsToWatch:     []string{"update", "delete"},
				NamespacesToIgnore: []string{"kube-system"},
			}
			reconciler.MinutesToKeep = "1"
			reconciler.HoursToKeep = "1"
			reconciler.DaysToKeep = "0"
		})

		It("should trigger manifest creation on valid update", func() {
			oldObj := &appsv1.Deployment{
				TypeMeta:   metav1.TypeMeta{Kind: "Deployment", APIVersion: "apps/v1"},
				ObjectMeta: metav1.ObjectMeta{Name: "test-app", Namespace: "default", Generation: 1},
			}
			newObj := oldObj.DeepCopy()
			newObj.Generation = 2

			e := event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}

			// Expect false return but side effect (TrashedResource creation)
			Expect(reconciler.HandleUpdate(e, fakeClient)).To(BeTrue())

			trList := &moxv1alpha1.TrashedResourceList{}
			Expect(fakeClient.List(context.Background(), trList)).To(Succeed())
			Expect(trList.Items).To(HaveLen(1))
		})

		It("should ignore updates with same generation", func() {
			oldObj := &appsv1.Deployment{
				TypeMeta:   metav1.TypeMeta{Kind: "Deployment", APIVersion: "apps/v1"},
				ObjectMeta: metav1.ObjectMeta{Name: "test-app", Namespace: "default", Generation: 1},
			}
			e := event.UpdateEvent{ObjectOld: oldObj, ObjectNew: oldObj.DeepCopy()}

			Expect(reconciler.HandleUpdate(e, fakeClient)).To(BeFalse())
			trList := &moxv1alpha1.TrashedResourceList{}
			Expect(fakeClient.List(context.Background(), trList)).To(Succeed())
			Expect(trList.Items).To(BeEmpty())
		})

		It("should create a TrashedResource after update resource", func() {
			oldObj := &appsv1.Deployment{
				TypeMeta:   metav1.TypeMeta{Kind: "Deployment", APIVersion: "apps/v1"},
				ObjectMeta: metav1.ObjectMeta{Name: "test-app", Namespace: "default", Generation: 1},
				Spec: appsv1.DeploymentSpec{
					Replicas: func(i int32) *int32 { return &i }(1),
				},
			}

			newObj := oldObj.DeepCopy()
			newObj.Spec.Replicas = func(i int32) *int32 { return &i }(2)
			newObj.Generation = 2

			e := event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}

			Expect(reconciler.HandleUpdate(e, fakeClient)).To(BeTrue())
			trList := &moxv1alpha1.TrashedResourceList{}
			Expect(fakeClient.List(context.Background(), trList)).To(Succeed())
			Expect(trList.Items).ToNot(BeEmpty())
		})

		It("should trigger manifest creation after delete resource", func() {
			obj := &appsv1.Deployment{
				TypeMeta:   metav1.TypeMeta{Kind: "Deployment", APIVersion: "apps/v1"},
				ObjectMeta: metav1.ObjectMeta{Name: "test-app", Namespace: "default", Generation: 1},
			}

			e := event.DeleteEvent{Object: obj, DeleteStateUnknown: false}

			// Expect true return and side effect (TrashedResource creation)
			Expect(reconciler.HandleDelete(e, fakeClient)).To(BeTrue())

			trList := &moxv1alpha1.TrashedResourceList{}
			Expect(fakeClient.List(context.Background(), trList)).To(Succeed())
			Expect(trList.Items).To(HaveLen(1))
		})
	})
})
