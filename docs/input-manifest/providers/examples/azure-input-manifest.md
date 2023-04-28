# Azure input manifest example

## Single provider, multi region cluster

```yaml
name: AzureExampleManifest

providers:
  azure:
    - name: azure-1
      # Service principal secret.
      clientSecret: Abcd~EFg~H6Ijkls~ABC15sEFGK54s78X~Olk9
      # ID of your subscription.
      subscriptionId: 6a4dfsg7-sd4v-f4ad-dsva-ad4v616fd512
      # ID of your tenancy.
      tenantId: 54cdafa5-sdvs-45ds-546s-df651sfdt614
      # ID of your service principal.
      clientId: 0255sc23-76we-87g6-964f-abc1def2gh3l

nodePools:
  dynamic:
    - name: control-azure
      providerSpec:
        # Name of the provider instance.
        name: azure-1
        # Location of the nodepool.
        region: West Europe
        # Zone of the nodepool.
        zone: 1
      count: 2
      # VM size name.
      serverType: Standard_B2s
      # URN of the image.
      image: Canonical:0001-com-ubuntu-minimal-jammy:minimal-22_04-lts:22.04.202212120

    - name: compute-1-azure
      providerSpec:
        # Name of the provider instance.
        name: azure-1
        # Location of the nodepool.
        region: Germany West Central
        # Zone of the nodepool.
        zone: 1
      count: 2
      # VM size name.
      serverType: Standard_B2s
      # URN of the image.
      image: Canonical:0001-com-ubuntu-minimal-jammy:minimal-22_04-lts:22.04.202212120
      storageDiskSize: 50

    - name: compute-2-azure
      providerSpec:
        # Name of the provider instance.
        name: azure-1
        # Location of the nodepool.
        region: West Europe
        # Zone of the nodepool.
        zone: 1
      count: 2
      # VM size name.
      serverType: Standard_B2s
      # URN of the image.
      image: Canonical:0001-com-ubuntu-minimal-jammy:minimal-22_04-lts:22.04.202212120
      storageDiskSize: 50

kubernetes:
  clusters:
    - name: azure-cluster
      version: v1.23.0
      network: 192.168.2.0/24
      pools:
        control:
          - control-azure
        compute:
          - compute-2-azure
          - compute-1-azure
```

## Multi provider, multi region clusters

```yaml
name: AzureExampleManifest

providers:
  azure:
    - name: azure-1
      # Service principal secret.
      clientSecret: Abcd~EFg~H6Ijkls~ABC15sEFGK54s78X~Olk9
      # ID of your subscription.
      subscriptionId: 6a4dfsg7-sd4v-f4ad-dsva-ad4v616fd512
      # ID of your tenancy.
      tenantId: 54cdafa5-sdvs-45ds-546s-df651sfdt614
      # ID of your service principal.
      clientId: 0255sc23-76we-87g6-964f-abc1def2gh3l

    - name: azure-2
      # Service principal secret.
      clientSecret: Efgh~ijkL~on43noi~NiuscviBUIds78X~UkL7
      # ID of your subscription.
      subscriptionId: 0965bd5b-usa3-as3c-ads1-csdaba6fd512
      # ID of your tenancy.
      tenantId: 55safa5d-dsfg-546s-45ds-d51251sfdaba
      # ID of your service principal.
      clientId: 076wsc23-sdv2-09cA-8sd9-oigv23npn1p2

nodePools:
  dynamic:
    - name: control-azure-1
      providerSpec:
        # Name of the provider instance.
        name: azure-1
        # Location of the nodepool.
        region: West Europe
        # Zone of the nodepool.
        zone: 1
      count: 1
      # VM size name.
      serverType: Standard_B2s
      # URN of the image.
      image: Canonical:0001-com-ubuntu-minimal-jammy:minimal-22_04-lts:22.04.202212120

    - name: control-azure-2
      providerSpec:
        # Name of the provider instance.
        name: azure-2
        # Location of the nodepool.
        region: Germany West Central
        # Zone of the nodepool.
        zone: 2
      count: 2
      # VM size name.
      serverType: Standard_B2s
      # URN of the image.
      image: Canonical:0001-com-ubuntu-minimal-jammy:minimal-22_04-lts:22.04.202212120

    - name: compute-azure-1
      providerSpec:
        # Name of the provider instance.
        name: azure-1
        # Location of the nodepool.
        region: Germany West Central
        # Zone of the nodepool.
        zone: 2
      count: 2
      # VM size name.
      serverType: Standard_B2s
      # URN of the image.
      image: Canonical:0001-com-ubuntu-minimal-jammy:minimal-22_04-lts:22.04.202212120
      storageDiskSize: 50

    - name: compute-azure-2
      providerSpec:
        # Name of the provider instance.
        name: azure-2
        # Location of the nodepool.
        region: West Europe
        # Zone of the nodepool.
        zone: 3
      count: 2
      # VM size name.
      serverType: Standard_B2s
      # URN of the image.
      image: Canonical:0001-com-ubuntu-minimal-jammy:minimal-22_04-lts:22.04.202212120
      storageDiskSize: 50

kubernetes:
  clusters:
    - name: azure-cluster
      version: v1.23.0
      network: 192.168.2.0/24
      pools:
        control:
          - control-azure-1
          - control-azure-2
        compute:
          - compute-azure-1
          - compute-azure-2
```
