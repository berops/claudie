# Claudie storage solution

## Longhorn

A Claudie-created cluster comes with the `longhorn` deployment preinstalled and ready to be used. By default, only **worker** nodes are used to store data.

Longhorn installed in the cluster is set up in such a way that it provides one default `StorageClass` called `longhorn`, which - if used - creates a volume that is then replicated across random nodes in the cluster. 

Besides the default storage class, Claudie can also create custom storage classes, which force persistent volumes to be created on a specific nodes based on the provider they have. In other words, you can use a specific provider to provision nodes for your storage needs, while using another provider for computing tasks.

## Example

To follow along, have a look at the reference [example input manifest file](../input-manifest/example.yaml).

When Claudie applies this input manifest, there are three storage classes installed:
- `longhorn` - the default storage class, which stores data on random nodes
- `longhorn-gcp-zone` - storage class that stores data on GCP nodes **only**
- `longhorn-hetzner-zone` - storage class, which stores data only on Hetzner nodes

## More information

For more information on how Longorn works you can check out [Longhorn's official documentation](https://longhorn.io/docs/1.3.0/what-is-longhorn/).