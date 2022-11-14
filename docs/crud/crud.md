# CRUD for Claudie

This document describes how user manages/communicates with Claudie deployed in the Kubernetes cluster.

The Claudie has a component called Frontend, which functions like an entrypoint to Claudie. The Frontend uses k8s-sidecar, which is configured to pull secrets with a label `claudie.io/input-manifest` and save it to the Frontend file system. The Frontend then picks it up and applies it to Claudie.
# Create

In order to create (apply) new input manifest, user needs to create a new secret in the namespace, where Claudie is deployed. This secret needs needs to have
- label `claudie.io/input-manifest`
- unique field name
  - **IMPORTANT**: If two secrets share the same data field name, the manifest saved by the k8s-sidecar will be overwritten, which may lead to deletion of the infrastructure.

### Example:

If you define an input manifest called `claudie-manifest.yaml` (see the example [here](../input-manifest/example.yaml)) you apply it by

1. Creating the secret by running
```
kubectl create secret generic input-manifest --from-file=input-manifest.yaml -n claudie
```

2. Labeling the secret with label claudie.io/input-manifest by running

```
kubectl label secret input-manifest claudie.io/input-manifest=my-fancy-manifest
```
# Read

The user and Claudie both share the single "source of truth" for the input manifests - the Kubernetes secrets. Created in the Claudie namespace, they are accessible by both user and Claudie. This forces users to have input manifests in IaC manner, and it can be easily configured for GitOps synchronization, i.e. via FluxCD.

# Update

If the user wishes to update the input manifest, they can edit/reapply the secret with the updated input manifest inside of it (the secret name and the data field name will stay the same). The k8s-sidecar will notice the change in the secret data, and will update the file inside the Frontend file system. The Frontend will then apply it to Claudie and the update of the defined infrastructure will be underway.

# Delete

If you wish to destroy your cluster along with the infra, you can remove the cluster definition block from the input-manifest and update the k8s secret respectively. If you wish you delete all the clusters defined in an input-manifest, you just simply need to delete the k8s secret containing the manifest. Both the events will trigger the deletion process. This deletion process will delete the current infra and it also deletes all data related to the particular input manifest.

# Outputs

So far, there is only one output Claudie has, which are kubeconfigs to your clusters. They are outputted in form of a secret, in the namespace, where claudie is deployed. The name of the secret follows the structure `<cluster-name>-<cluster-hash>-kubeconfig`. You can access it by outputting it and decoding it from base64 encoding. 

Example of how to decode it:

`kubectl get secrets -n claudie <cluster-name>-<cluster-hash>-kubeconfig -o jsonpath='{.data.secretdata}' | base64 -d > your_kubeconfig.yaml`
