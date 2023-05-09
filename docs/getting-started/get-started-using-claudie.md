# Get started using Claudie

## First steps

1. Deploy Claudie Kubernetes [manifests/claudie](https://github.com/berops/claudie/tree/master/manifests/claudie) into a Kubernetes cluster:
   ```sh
   kubectl apply -k manifests/claudie
   ```
2. Provide your own manifest via a Kubernetes Secret, but before that please have a look at our [reference example input manifest](../input-manifest/example.md) to explore what's possible.

To see in detail how to correctly apply the manifest into Claudie, please refer to the [CRUD](../crud/crud.md) document.

After the input manifest is successfully applied, the kubeconfig to your newly
built clusters is output as a secret in the `claudie` namespace with a name in
the form of `<cluster-name>-<cluster-hash>-kubeconfig`.