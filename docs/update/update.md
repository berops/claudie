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
      version: v1.25.0
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
      version: v1.26.0
      network: 192.168.2.0/24
      pools:
        ...
```

When re-applied this will trigger a new workflow for the cluster that will result in the updated kubernetes version.

!!! note "Downgrading a version is not supported once you've upgraded a cluster to a newer version"

## Updating the OS image

Similarly, as to how the kubernetes version is updated you can update the OS image by just replacing
it with a new version in the desired dynamic nodepool.

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
  image: ubuntu-20.04
...
```

```yaml
# new version
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

When re-applied this will trigger a new workflow for the cluster that will result in the updated OS image version.

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
- name: hetzner
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

!!! warning "Rollout Update"
         When making changes to the nodepools the newly started workflow will not execute a rollout replacement,
         it will re-create the instances in all places where the nodepool is referenced. It's possible to achieve a rollout strategy by firstly adding a new nodepool with the desired parameters waiting for it to be build and then deleting the references of the old nodepool and apply.
