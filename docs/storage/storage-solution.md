# Claudie storage proposal

## Longhorn

Claudie created cluster come with the longhorn deployment preinstalled and ready to be used. By default, only **worker** nodes are used to store data.

Longhorn installed in the cluster is set up in a way, that it provides one default `StorageClass` called `longhorn`, which if used, will create a volume on random node in a cluster. However, Claudie creates a few new storage classes, which are set up in a way, that allows the data to be stored only on a nodes in a specific node provider. 

## Example

### Input manifest

```yaml
name: Storage-demo

providers:
  - name: hetzner
    credentials: abcd1234
  - name: gcp
    credentials: '{}'
    gcp_project: project-id
    
nodePools:
  dynamic:
    - name: hetzner-control
      provider:
        hetzner:
          region: nbg1
          zone: nbg1-dc3
      count: 2
      server_type: cpx11
      image: ubuntu-20.04
      disk_size: 50
    - name: hetzner-compute
      provider:
        hetzner:
          region: nbg1
          zone: nbg1-dc3
      count: 2
      server_type: cpx11
      image: ubuntu-20.04
      disk_size: 50
    - name: gcp-control
      provider:
        gcp:
          region: europe-west1
          zone: europe-west1-c
      count: 3
      server_type: e2-medium
      image: ubuntu-os-cloud/ubuntu-2004-focal-v20220610
      disk_size: 50
    - name: gcp-compute
      provider:
        gcp:
          region: europe-west1
          zone: europe-west1-c
      count: 2
      server_type: e2-small
      image: ubuntu-os-cloud/ubuntu-2004-focal-v20220610
      disk_size: 50

kubernetes:
  clusters:
    - name: dev-cluster
      version: v1.22.0
      network: 192.168.2.0/24
      pools:
        control:
          - hetzner-control
          - gcp-control
        compute:
          - hetzner-compute
          - gcp-compute
```

When Claudie will apply this input manifest, there will be three storage classes installed. 

- `longhorn` - default storage class, which will store data on a random node
- `longhorn-gcp-zone` - storage class which will store data only on a gcp nodes
- `longhorn-hetzner-zone` - storage class which will store data only on a hetzner nodes

For more information how Longorn works, see the [official documentation](https://longhorn.io/docs/1.3.0/what-is-longhorn/)