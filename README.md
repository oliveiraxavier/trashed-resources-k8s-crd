# Trashed resources

## Description

This project introduces a custom CRD designed to enhance cluster safety by
tracking objects that are updated or removed from your Kubernetes Cluster. It
acts as a recycle bin, ensuring that deleted or modified items are temporarily
(based on configuration) stored to be udited or recovered. [WIP]

## Features

1 - Tracks deletions and updates across the cluster.

2 - Provides a sort of "Recycle Bin" for Kubernetes resources.

3 - Cli plugin to interact with the trashedresources (restore or prune) via kubectl.

## CRD installation

```sh
kubectl apply -f https://raw.githubusercontent.com/oliveiraxavier/trashed-resources-k8s-crd/1.0.0/dist/install.yaml
```

## Edit default Configmap configuration

By default, the configmap is:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: trashed-resources-trashedresources-config
  namespace: trashed-resources-system
data:
  actionsToObserve: delete #or update
  kindsToObserve: Deployment; Secret; ConfigMap
  namespacesToIgnore: istio-system; kube-node-lease; kube-public; kube-system
  daysToKeep: "0"
  hoursToKeep: "0"
  minutesToKeep: "10"
```

You must configure it according to your scenario. Restart controller pod after
change this configmap.

### How trashedresources is generated?

Eg. After delete a deployment, a new trashed resource is generated. The generated
trashedresource name is composed of the "trashed-action + resource type + resource name + date + time"
(eg. trashed-deleted-deployment-nginx-20260301-230159 or trashed-updated-deployment-nginx-20260301-230159)

### Install plugin

1 - With curl

```sh
curl https://raw.githubusercontent.com/oliveiraxavier/trashed-resources-k8s-crd/1.0.0/bin/kubectl-trashedresources \
  -O --output-dir  ~/.local/bin
```

- Add it to your path if necessary

```sh
echo -e '\nexport PATH=~/.local/bin:$PATH' >> ~/.bashrc
```

2 - From source

```sh
make createcmdbin
export PATH=$PATH:$(pwd)/bin
# Or set in your ~/.bashrc (Linux)
echo export PATH=$PATH:$(pwd)/bin >> ~/.bashrc
# Or set in your ~/.bashrc (Mac)
echo export PATH=$PATH:$(pwd)/bin >> ~/.zshrc
```

### Interact via cli (as plugin) with kubectl

```sh
# For trashed-resource named trashed-deleted-deployment-nginx-deployment-20260301-230159
kubectl trashedresources prune --name trashed-deleted-deployment-nginx-deployment-20260301-230159 --namespace default

# For trashed-resource named trashed-deleted-deployment-nginx-deployment-20260301-230159  and age older than 12 minutes
kubectl trashedresources prune --name trashed-deleted-deployment-nginx-deployment-20260301-230159 --older-than 12m --namespace default

# For trashed-resource named trashed-deleted-deployment-nginx-deployment-20260301-230159  and age older than 1 hour
kubectl trashedresources prune --name trashed-deleted-deployment-nginx-deployment-20260301-230159 --older-than 1h --namespace default

# For trashed-resource named trashed-deleted-deployment-nginx-deployment-20260301-230159  and age older than 1 day
kubectl trashedresources prune --name trashed-deleted-deployment-nginx-deployment-20260301-230159 --older-than 1d --namespace default

# For all trashed-resources in the cluster with age older than 1 day
kubectl trashedresources prune --older-than 1d --namespace default
```

## Getting Started to contribute or test/install from source

### Prerequisites

- go version v1.26.1
- docker version 29.3.0+.
- kubectl version v1.32.0+.
- Access to a Kubernetes v1.11.3+ cluster.

## Run locally

```sh
make run
```

### To Deploy on the cluster

**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/trashed-resources-controller:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/trashed-resources-controller:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall

**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/trashed-resources:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

### By providing a Helm Chart

1 - Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v1-alpha
```

2 - See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing

// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
