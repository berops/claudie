# Hetzner input manifest example

## Single provider, multi region cluster

```yaml
name: HetznerExampleManifest

providers:
  hetzner:
    - name: hetzner-1
      # API access token.
      credentials: kslISA878a6etYAfXYcg5iYyrFGNlCxcICo060HVEygjFs21nske76ksjKko21lp

nodePools:
  dynamic:
    - name: control-hetzner
      providerSpec:
        # Name of the provider instance.
        name: hetzner-1
        # Region of the nodepool.
        region: hel1
        # Datacenter of the nodepool.
        zone: hel1-dc2
      count: 1
      # Machine type name.
      serverType: cpx11
      # OS image ID for ubuntu-22.04.
      image: "67794396"
      diskSize: 50

    - name: compute-1-hetzner
      providerSpec:
        # Name of the provider instance.
        name: hetzner-1
        # Region of the nodepool.
        region: fsn1
        # Datacenter of the nodepool.
        zone: fsn1-dc14
      count: 2
      # Machine type name.
      serverType: cpx11
      # OS image ID for ubuntu-22.04.
      image: "67794396"
      diskSize: 50

    - name: compute-2-hetzner
      providerSpec:
        # Name of the provider instance.
        name: hetzner-1
        # Region of the nodepool.
        region: nbg1
        # Datacenter of the nodepool.
        zone: nbg1-dc3
      count: 2
      # Machine type name.
      serverType: cpx11
      # OS image ID for ubuntu-22.04.
      image: "67794396"
      diskSize: 50

kubernetes:
  clusters:
    - name: hetzner-cluster
      version: v1.23.0
      network: 192.168.2.0/24
      pools:
        control:
          - control-hetzner
        compute:
          - compute-1-hetzner
          - compute-2-hetzner
```

## Multi provider, multi region clusters

```yaml
name: HetznerExampleManifest

providers:
  hetzner:
    - name: hetzner-1
      # API access token.
      credentials: kslISA878a6etYAfXYcg5iYyrFGNlCxcICo060HVEygjFs21nske76ksjKko21lp

    - name: hetzner-2
      # API access token.
      credentials: kslIIOUYBiuui7iGBYIUiuybpiUB87bgPyuCo060HVEygjFs21nske76ksjKko21l

nodePools:
  dynamic:
    - name: control-hetzner-1
      providerSpec:
        # Name of the provider instance.
        name: hetzner-1
        # Region of the nodepool.
        region: hel1
        # Datacenter of the nodepool.
        zone: hel1-dc2
      count: 1
      # Machine type name.
      serverType: cpx11
      # OS image ID for ubuntu-22.04.
      image: "67794396"
      diskSize: 50

    - name: control-hetzner-2
      providerSpec:
        # Name of the provider instance.
        name: hetzner-2
        # Region of the nodepool.
        region: fsn1
        # Datacenter of the nodepool.
        zone: fsn1-dc14
      count: 2
      # Machine type name.
      serverType: cpx11
      # OS image ID for ubuntu-22.04.
      image: "67794396"
      diskSize: 50

    - name: compute-hetzner-1
      providerSpec:
        # Name of the provider instance.
        name: hetzner-1
        # Region of the nodepool.
        region: fsn1
        # Datacenter of the nodepool.
        zone: fsn1-dc14
      count: 2
      # Machine type name.
      serverType: cpx11
      # OS image ID for ubuntu-22.04.
      image: "67794396"
      diskSize: 50

    - name: compute-hetzner-2
      providerSpec:
        # Name of the provider instance.
        name: hetzner-2
        # Region of the nodepool.
        region: nbg1
        # Datacenter of the nodepool.
        zone: nbg1-dc3
      count: 2
      # Machine type name.
      serverType: cpx11
      # OS image ID for ubuntu-22.04.
      image: "67794396"
      diskSize: 50

kubernetes:
  clusters:
    - name: hetzner-cluster
      version: v1.23.0
      network: 192.168.2.0/24
      pools:
        control:
          - control-hetzner-1
          - control-hetzner-2
        compute:
          - compute-hetzner-1
          - compute-hetzner-2
```
