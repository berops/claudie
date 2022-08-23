# Claudie

<!-- Basic info what claudie is -->
Claudie is a simple solution for managing your multi-cloud kubernetes clusters. From node creation, cluster creation to loadbalancing and DNS set up it makes the kubernetes management a piece of cake.

# Features
<!-- Why is Claudie coolest thing ever -->
### Claudie management via IaC 

Declaratively define your infrastructure with a simple, easy to understand YAML [manifest](./docs/input-manifest/example.yaml).

### Fast scale-up/scale-down of your infrastructure
To scale-up or scale-down, simply change a few lines in the input manifest and Claudie will take care of the rest.

### Simple multi-cloud set up
Claudie is built with multi-cloud infrastructure in mind. Simply provide credentials to your cloud projects and watch how the infra is being build in front of you.
### Loadbalancing
TODO

### DNS management
TODO

# Get started using the Claudie

Claudie was designed to be run in any standard kubernetes cluster. Therefore, the easiest way to get started with a claudie is to deploy it to some already built cluster. The deployments are accessible in [manifests](https://github.com/Berops/platform/tree/master/manifests/claudie) directory. To deploy it simply create a namespace by running

```
kubectl create namespace claudie
```
and then deploy Claudie by running

```
kustomize build | kubectl apply -f -
```

To input the manifest into the claudie, you need to create a secret, which holds the input manifest defined by you.

Example:

If you define an input manifest called `claudie-manifest.yaml` (see the example [here](https://github.com/Berops/platform/blob/master/docs/input-manifest/example.yaml)) you apply it by 

1. Creating the secret by running
```
kubectl create secret generic input-manifest --from-file=input-manifest.yaml -n claudie
```

2. Labeling the secret with label `claudie.io/input-manifest` by running
```
kubectl label secret input-manifest claudie.io/input-manifest=my-fancy-manifest
```
This will be then picked up by the Claudie and will be automatically applied.
To see in full details how you manage the Claudie, please refer to [CRUD](./docs/crud/crud.md) document.

# Get involved

<!-- Contributor guidelines -->
Everyone is more than welcome to open an issue, a PR or to start a discussion. For more information about contributing please read the [contribution guidelines](./docs/contributing/contributing.md).

# Bug reports
When you encounter a bug, please create a new [issue](https://github.com/Berops/platform/issues/new/choose) and use bug template. Before you submit, please check

- If the issue you want to open is not a duplicate
- You submitted any errors and concise way to reproduce the issue
- The input manifest you used (be careful not to include your cloud credentials) 

# Roadmap
<!-- Add a roadmap for claudie so users know which features are being worked on and which will in future -->
To see the vision behind the Claudie, please refer to the [roadmap](./docs/roadmap/roadmap.md) document.

# License

Licensed under the [Apache License, Version 2.0](LICENSE)