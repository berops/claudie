# Updating Claudie

In this section we'll describe how you can update resources that claudie creates based
on changes in the manifest.

## Updating Kubernetes Version

Updating the Kubernetes version is as easy as incrementing the version
in the Input Manifest of the already build cluster.

```yaml
# old version
...
kubernetes:
  clusters:
    - name: claudie-cluster
      version: v1.30.0
      network: 192.168.2.0/24
      pools:
        ...
```

```yaml
# new version
...
kubernetes:
  clusters:
    - name: claudie-cluster
      version: 1.31.0
      network: 192.168.2.0/24
      pools:
        ...
```

When re-applied this will trigger a new workflow for the cluster that will result in the updated kubernetes version.

!!! note "Downgrading a version is not supported once you've upgraded a cluster to a newer version"

# Updating Dynamic Nodepool

Nodepools specified in the InputManifest are immutable. Once created, they cannot be updated/changed. This decision was made to force the user to perform a rolling update by first deleting the nodepool and replacing it with a new version with the new desired state. A couple of examples are listed below.

## Updating the OS image

```yaml
# old version
...
- name: hetzner
  providerSpec:
    name: hetzner-1
    region: fsn1
    zone: fsn1-dc14
  count: 1
  serverType: cpx11
  image: ubuntu-22.04
...
```

```yaml
# new version
...
- name: hetzner-1 # NOTE the different name.
  providerSpec:
    name: hetzner-1
    region: fsn1
    zone: fsn1-dc14
  count: 1
  serverType: cpx11
  image: ubuntu-24.04
...
```

When re-applied this will trigger a new workflow for the cluster that will result first in the addition of the new nodepool and then the deletion of the old nodepool. 

## Changing the Server Type of a Dynamic Nodepool

The same concept applies to changing the server type of a dynamic nodepool.

```yaml
# old version
...
- name: hetzner
  providerSpec:
    name: hetzner-1
    region: fsn1
    zone: fsn1-dc14
  count: 1
  serverType: cpx11
  image: ubuntu-22.04
...
```

```yaml
# new version
...
- name: hetzner-1 # NOTE the different name.
  providerSpec:
    name: hetzner-1
    region: fsn1
    zone: fsn1-dc14
  count: 1
  serverType: cpx21
  image: ubuntu-22.04
...
```

When re-applied this will trigger a new workflow for the cluster that will result in the updated server type of the nodepool.