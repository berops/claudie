# Claudie storage proposal

## Longhorn

Claudie created cluster comes with the longhorn deployment preinstalled and ready to be used. By default, only **worker** nodes are used to store data.

Longhorn installed in the cluster is set up in a way, it provides one default `StorageClass` called `longhorn`, which if used, will create a volume on random node in a cluster. 

Other than default storage class, Claudie creates custom storage classes, which force persistent volumes to be created on a specific nodes based on the provider they have. In other words, you can use a specific provider to provision nodes for your storage needs, while using other provider for computing tasks.

## Example

### [Input manifest](../input-manifest/example.yaml)

When Claudie will apply this input manifest, there will be three storage classes installed. 

- `longhorn` - default storage class, which will store data on a random node
- `longhorn-gcp-zone` - storage class which will store data only on a gcp nodes
- `longhorn-hetzner-zone` - storage class which will store data only on a hetzner nodes

For more information how Longorn works, see the [official documentation](https://longhorn.io/docs/1.3.0/what-is-longhorn/)