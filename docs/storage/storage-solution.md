# Claudie storage proposal

## Longhorn

Claudie created cluster comes with the longhorn deployment preinstalled and ready to be used. By default, only **worker** nodes are used to store data.

Longhorn installed in the cluster is set up in a way, it provides one default `StorageClass` called `longhorn`, which if used, will create a volume which is replicated across random nodes in a cluster. 

Other than default storage class, Claudie creates custom storage classes, which force persistent volumes to be created on a specific nodes based on their provider. In other words, you can use a specific provider to provision nodes for your storage needs, while using other provider for computing tasks.

## Example

### [Input manifest](../input-manifest/example.yaml)

When Claudie will apply this input manifest, there will be a few storage classes installed. 

- `longhorn` - default storage class, which will store data on a random node
- `longhorn-<provider>-zone` - storage class which will store data only on a nodes from specified provider. To see the list of supported providers, please refer [here](../input-manifest/input-manifest.md#providers)

For more information how Longhorn works, see the [official documentation](https://longhorn.io/docs/1.3.0/what-is-longhorn/)