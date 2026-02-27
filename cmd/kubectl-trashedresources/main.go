package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	moxv1alpha1 "trashed-resources/api/v1alpha1"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	// Register Kubernetes core scheme and your CRD
	_ = moxv1alpha1.AddToScheme(scheme)
	_ = metav1.AddMetaToScheme(scheme)
}

func main() {
	// Default Kubectl configuration (to get namespace, kubeconfig, etc.)
	kubernetesConfigFlags := genericclioptions.NewConfigFlags(true)

	rootCmd := &cobra.Command{
		Use:   "kubectl-trashedresources",
		Short: "Plugin to manage TrashedResources",
		Long:  `CLI tool to restore and prune TrashedResources in the cluster.`,
	}

	// Add global k8s flags (e.g. -n namespace)
	kubernetesConfigFlags.AddFlags(rootCmd.PersistentFlags())

	// --- RESTORE Command ---
	restoreCmd := &cobra.Command{
		Use:   "restore [NAME]",
		Short: "Restores a deleted resource from a TrashedResource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceName := args[0]
			ns, _, err := kubernetesConfigFlags.ToRawKubeConfigLoader().Namespace()
			if err != nil {
				return err
			}

			k8sClient, err := getClient(kubernetesConfigFlags)
			if err != nil {
				return err
			}

			return restoreResource(k8sClient, resourceName, ns)
		},
	}
	rootCmd.AddCommand(restoreCmd)

	// --- PRUNE Command Delete  by age  or name
	var olderThan string
	var name string
	pruneCmd := &cobra.Command{
		Use:   "prune",
		Short: "Deletes TrashedResources older than a specified duration",
		Long: `Example: kubectl trashedresources prune --older-than 1d
		or kubectl trashedresources prune --name trashed-deployment-myapp-12345`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if olderThan == "" && name == "" {
				return fmt.Errorf("flag --older-than or --name is required")
			}
			hasArgumentDuration := false
			duration := time.Duration(0)
			var err error

			if olderThan != "" {
				duration, err = time.ParseDuration(olderThan)
				hasArgumentDuration = true
				if err != nil {
					// Try to support "1d" which native Go time.ParseDuration sometimes doesn't support depending on version/lib,
					// but here we assume standard Go format (1h, 10m, 24h).
					// If days are needed, use 24h, 48h, etc.
					if strings.HasSuffix(olderThan, "d") {
						daysStr := strings.TrimSuffix(olderThan, "d")
						days, err := time.ParseDuration(daysStr + "h")
						if err == nil {
							duration = days * 24
						} else {
							return fmt.Errorf("invalid time format: %v", err)
						}

					} else {
						return fmt.Errorf("invalid time format (use 10m, 5h, 24h): %v", err)
					}
				}
			}
			ns, _, err := kubernetesConfigFlags.ToRawKubeConfigLoader().Namespace()
			if err != nil {
				// If namespace is not specified, assume all (depending on config) or default
				ns = ""
			}

			k8sClient, err := getClient(kubernetesConfigFlags)
			if err != nil {
				return err
			}

			return pruneResources(k8sClient, ns, duration, hasArgumentDuration, name)
		},
	}
	pruneCmd.Flags().StringVar(&olderThan, "older-than", "", "Duration to consider old (e.g. 14m, 11h, 24h)")
	pruneCmd.Flags().StringVar(&name, "name", "", "Name of the TrashedResource to delete")

	rootCmd.AddCommand(pruneCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func getClient(flags *genericclioptions.ConfigFlags) (client.Client, error) {
	cfg, err := flags.ToRESTConfig()
	if err != nil {
		// Fallback to in-cluster or local default if flags fail
		cfg, err = config.GetConfig()
		if err != nil {
			return nil, err
		}
	}
	return client.New(cfg, client.Options{Scheme: scheme})
}

func restoreResource(c client.Client, name, namespace string) error {
	ctx := context.Background()

	// 1. Get the TrashedResource
	trashed := &moxv1alpha1.TrashedResource{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, trashed)
	if err != nil {
		return fmt.Errorf("failed to find TrashedResource %s/%s: %v", namespace, name, err)
	}

	fmt.Printf("Restoring resource from: %s\n", trashed.Name)

	// 2. Convert spec.data (YAML string) to Unstructured
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(trashed.Spec.Data), 4096)
	newObj := &unstructured.Unstructured{}
	if err := decoder.Decode(newObj); err != nil {
		return fmt.Errorf("failed to decode resource data: %v", err)
	}

	newObj.SetUID("")
	newObj.SetResourceVersion("")
	newObj.SetGeneration(newObj.GetGeneration())
	newObj.SetGeneration(newObj.GetGeneration())
	newObj.SetCreationTimestamp(newObj.GetCreationTimestamp())
	newObj.SetOwnerReferences(newObj.GetOwnerReferences())
	newObj.SetManagedFields(newObj.GetManagedFields())

	err = c.Create(ctx, newObj)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return fmt.Errorf("resource %s %s/%s already exists", newObj.GetKind(), newObj.GetNamespace(), newObj.GetName())
		}
		return fmt.Errorf("failed to create restored resource: %v", err)
	}

	fmt.Printf("Success! Resource %s %s/%s restored.\n", newObj.GetKind(), newObj.GetNamespace(), newObj.GetName())
	err = c.Delete(ctx, trashed, client.PropagationPolicy(metav1.DeletePropagationBackground))
	if err != nil {
		return fmt.Errorf("failed to delete TrashedResource %s/%s: %v. You should manually delete it", namespace, name, err)
	}
	return nil
}

func pruneResources(c client.Client, namespace string, olderThan time.Duration,
	hasArgumentDuration bool, name string) error {
	ctx := context.Background()
	list := &moxv1alpha1.TrashedResourceList{}

	opts := []client.ListOption{}
	if namespace != "" {
		opts = append(opts, client.InNamespace(namespace))
	}
	ignoreAge := !hasArgumentDuration
	if name != "" {
		opts = append(opts, client.MatchingFields(map[string]string{"metadata.name": name}))
	}

	if err := c.List(ctx, list, opts...); err != nil {
		return fmt.Errorf("failed to list TrashedResources: %v", err)
	}

	cutoffTime := time.Now().Add(-olderThan)
	deletedCount := 0

	if olderThan != 0 {
		fmt.Printf("Searching for TrashedResources created before %s (older-than %s)\n",
			cutoffTime.Format(time.DateTime),
			olderThan)
	}

	if name != "" {
		fmt.Printf("Searching for TrashedResources named as %s\n", name)
	}
	for _, tr := range list.Items {
		// Check age based on TrashedResource CreationTimestamp
		if tr.CreationTimestamp.Time.Before(cutoffTime) || ignoreAge {

			fmt.Printf("Deleting %s/%s (Created at: %s)\n", tr.Namespace, tr.Name, tr.CreationTimestamp.Format(time.DateTime))

			if err := c.Delete(ctx, &tr); err != nil {
				fmt.Printf("ERROR deleting %s: %v\n", tr.Name, err)
			} else {
				deletedCount++
			}
		}
	}
	fmt.Printf("Total deleted: %d\n", deletedCount)

	return nil
}
