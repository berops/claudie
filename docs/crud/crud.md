# CRUD for Claudie
This document describes how the user manages/communicates with Claudie deployed in a Kubernetes cluster.

Claudie has a component called Frontend, which functions like an entrypoint to Claudie. Frontend uses `k8s-sidecar`, which is configured to pull secrets with a label `claudie.io/input-manifest` and save it to Frontend's file system. Frontend then picks it up and applies it to Claudie.

# Create
In order to create (apply) a new input manifest, the user needs to create a new secret in the namespace where Claudie is deployed. This secret needs needs to have:
- a label `claudie.io/input-manifest`
- a unique field name
  - **IMPORTANT**: If two secrets share the same data field name, the manifest saved by `k8s-sidecar` will be overwritten, which may lead to deletion of the infrastructure.

### Example:
If you define an input manifest called `claudie-manifest.yaml` (see the example [here](../input-manifest/example.yaml)) you can apply it by:
1. Creating the secret by running
```
kubectl create secret generic input-manifest --from-file=input-manifest.yaml -n claudie
```

2. Labeling the secret with label `claudie.io/input-manifest` by running
```
kubectl label secret input-manifest claudie.io/input-manifest=my-fancy-manifest -n claudie
```

# Read
The user and Claudie both share the single "source of truth" for the input manifests - Kubernetes secrets. Created in *the Claudie namespace*, they are accessible by both user and Claudie.
This forces users to store input manifests in an [IaC](https://en.wikipedia.org/wiki/Infrastructure_as_code) manner and can easily be configured for GitOps synchronization (i.e. via FluxCD).

# Update
If the user wishes to update the input manifest, they can edit/reapply the secret with the updated input manifest inside of it (the secret name and the data field name will stay the same). `k8s-sidecar` will notice the change in the secret data, and will update the file inside Frontend's file system. Frontend will then apply it to Claudie and the update of the defined infrastructure will be underway.

# Delete
If you wish to destroy your cluster along with the infrastructure, you can remove the cluster definition block from the input-manifest and update the k8s secret accordinglt.
If you wish you delete all the clusters defined in an input-manifest, you simply need to delete the k8s secret containing the manifest. Both events will trigger the deletion process. This deletion process will delete the current infrastructure and it also deletes all data related to the particular input manifest.

# Outputs
Claudie outputs two secret after a successful run of the manifest, which are kubeconfigs and cluster metadata to your clusters. They are output in form of a secret, in the namespace where Claudie is deployed. The name of the secret follows the structure `<cluster-name>-<cluster-hash>-kubeconfig/metadata`. The secret can be accessed by printing+decoding it from `base64`. 

Example of how to decode the secred:
`kubectl get secrets -n claudie <cluster-name>-<cluster-hash>-kubeconfig -o jsonpath='{.data.secretdata}' | base64 -d > your_kubeconfig.yaml`
