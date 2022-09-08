# Claudie

![Build](https://github.com/Berops/claudie/actions/workflows/CD-pipeline-dev.yml/badge.svg)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

Platform for managing multi-cloud Kubernetes clusters.

# Features

### Manage multi-cloud Kubernetes clusters

Create fully-featured Kubernetes clusters composed of multiple different public Cloud providers in an easy and secure manner.
Simply insert credentials to your cloud projects, define your cluster, and watch how the infra spawns right in front of you.

### Management via IaC 

Declaratively define your infrastructure with a simple, easy to understand YAML [syntax](./docs/input-manifest/input-manifest.md).
See example [manifest](./docs/input-manifest/example.yaml).

### Fast scale-up/scale-down of your infrastructure

To scale-up or scale-down, simply change a few lines in the input manifest and Claudie will take care of the rest in the matter of minutes.

### Loadbalancing 

Claudie has its own managed loadbalancing solution, which you can use for Ingresses, for the Kubernetes API server, or generally anything. See [LB docs](https://github.com/Berops/claudie/tree/master/docs/loadbalancing).

### Persistent storage volumes 

Claudie comes with a pre-configured storage solution, with ready-to-use Storage Classes. See [Storage docs](https://github.com/Berops/claudie/tree/master/docs/storage).

# Get started using the Claudie

Deploy Claudie Kubernetes [manifests](https://github.com/Berops/claudie/tree/master/manifests/claudie) into a Kubernetes cluster.

```
kustomize build | kubectl apply -f -
```

Lastly, provide your own manifest via a Kubernetes Secret.

Example of the input manifest can be found [here](https://github.com/Berops/claudie/blob/master/docs/input-manifest/example.yaml).

To see in detail how you apply the manifest into Claudie, please refer to the [CRUD](./docs/crud/crud.md) document.

# Get involved

<!-- Contributor guidelines -->
Everyone is more than welcome to open an issue, a PR or to start a discussion. 

For more information about contributing please read the [contribution guidelines](./docs/contributing/contributing.md).

If you want to have a chat with us, feel free to join our [slack workspace](https://claudie-workspace.slack.com).

# Roadmap
<!-- Add a roadmap for claudie so users know which features are being worked on and which will in future -->
To see the vision behind the Claudie, please refer to the [roadmap](./docs/roadmap/roadmap.md) document.

