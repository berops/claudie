# CRUD for Claudie

This document describes how the user manages/communicates with Claudie deployed in a Kubernetes cluster.

Claudie has a component called Frontend, which functions like an entrypoint to Claudie. Frontend uses k8s Watch API to continuously pull secrets with a label `claudie.io/input-manifest` and save them to Claudie database.

## Create

In order to create (apply) a new input manifest, the user needs to create a new secret in the namespace where Claudie is deployed. This secret needs needs to have:

- a label `claudie.io/input-manifest`
- a unique field name
  - **IMPORTANT**: We highly recommend to have single input manifest per secret.

### Example

If you define an input manifest called `claudie-manifest.yaml` (see the example [here](../input-manifest/example.yaml)) and apply it by:

1. Creating the secret by running

    ```sh
    kubectl create secret generic input-manifest --from-file=input-manifest.yaml -n claudie
    ```

2. Labeling the secret with label `claudie.io/input-manifest` by running

    ```sh
    kubectl label secret input-manifest claudie.io/input-manifest=my-fancy-manifest -n claudie
    ```

## Read

The user and Claudie both share a single "source of truth" for the input manifests - Kubernetes secrets. Created in *the Claudie namespace*, they are accessible by both the user and Claudie.
This makes users store input manifests in an [IaC](https://en.wikipedia.org/wiki/Infrastructure_as_code) manner and can easily be configured for GitOps synchronization (i.e. via FluxCD).

## Update

When you want to update the input manifest, you can edit/reapply the secret with the updated input manifest inside of it (the secret name and the data field name will stay the same). Frontend notices the change in the secret data and subsequently notifies the Claudie about change made to the input manifest.

## Delete

If you wish to destroy your cluster along with the infrastructure, you can remove the cluster definition block from the input-manifest and update the k8s secret accordingly.
If you wish to delete all of the clusters defined in an input-manifest, you simply need to delete the k8s secret containing the manifest. Both events will trigger the deletion process. This process deletes the current infrastructure and it also deletes all data related to the particular input manifest.

## Outputs

Claudie outputs two secrets in the namespace where it is deployed, after a successful run of the (input) manifest:

- kubeconfig,
- cluster metadata to your clusters.

The names of the secrets are derived as follows: `<cluster-name>-<cluster-hash>-{kubeconfig,metadata}`. The secrets can be accessed by printing and `base64`-decoding them.

Example of how to decode a kubeconfig from secret:

```sh
kubectl get secrets -n claudie -l claudie.io/cluster=<cluster name> claudie.io/project=<project name> claudie.io/output=kubeconfig -o jsonpath='{.data.kubeconfig}' | base64 -d > your_kubeconfig.yaml
```

Each claudie output will have following labels:

| Key                     | Value                                           |
| ----------------------- | ----------------------------------------------- |
| `claudie.io/project`    | Name of the project.                            |
| `claudie.io/cluster`    | Name of the cluster.                            |
| `claudie.io/cluster-id` | ID of the cluster.                              |
| `claudie.io/output`     | Output type, either `kubeconfig` or `metadata`. |
