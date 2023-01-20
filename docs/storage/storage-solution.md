# Claudie storage solution

## Concept

Running stateful workloads is a complex task, even more so when considering the {multi,hybrid}-cloud environment. Claudie therefore needs to be able to accommodate stateful workloads, regardless of the underlying infrastructure providers.

Claudie orchestrates storage on the kubernetes cluster nodes by creating one storage cluster across multiple providers. This storage cluster has a series of `zones`, one for each cloud provider. Each `zone` then stores its own persistent volume data.

## Longhorn

A Claudie-created cluster comes with the `longhorn` deployment preinstalled and ready to be used. By default, only **worker** nodes are used to store data.

Longhorn installed in the cluster is set up in such a way that it provides one default `StorageClass` called `longhorn`, which - if used - creates a volume that is then replicated across random nodes in the cluster. 

Besides the default storage class, Claudie can also create custom storage classes, which force persistent volumes to be created on a specific nodes based on the provider they have. In other words, you can use a specific provider to provision nodes for your storage needs, while using another provider for computing tasks.

## Example

To follow along, have a look at the reference [example input manifest file](../input-manifest/example.yaml).

When Claudie applies this input manifest, the following storage classes are installed:
- `longhorn` - the default storage class, which stores data on random nodes
- `longhorn-<provider>-zone` - storage class, which stores data only on nodes of the specified providier (see the [list of supported providers](../input-manifest/input-manifest.md#providers))

For more information on how Longorn works you can check out [Longhorn's official documentation](https://longhorn.io/docs/1.3.0/what-is-longhorn/).
