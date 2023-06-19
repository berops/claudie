# Azure
Azure provider requires you to input `clientSecret`, `subscriptionId`, `tenantId`, and `clientId`.

## Compute and DNS example
```yaml
providers:
  azure:
    - name: azure-1
      clientSecret: Abcd~EFg~H6Ijkls~ABC15sEFGK54s78X~Olk9
      subscriptionId: 6a4dfsg7-sd4v-f4ad-dsva-ad4v616fd512 # all resources you define will be charged here
      tenantId: 54cdafa5-sdvs-45ds-546s-df651sfdt614
      clientId: 0255sc23-76we-87g6-964f-abc1def2gh3l
```

## Create Azure credentials
### Prerequisites
1. Install Azure CLI by following [this guide](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli).
2. Login to Azure [this guide](https://learn.microsoft.com/en-us/cli/azure/authenticate-azure-cli).

### Creating Azure credentials for Claudie
1. Login to Azure with the following command:
    ```bash
    az login
    ```

2. Permissions file for the new role that claudie service principal will use:
    ```bash
    cat > policy.json <<EOF
    {
       "Name":"Resource Group Management",
       "Id":"bbcd72a7-2285-48ef-bn72-f606fba81fe7",
       "IsCustom":true,
       "Description":"Create and delete Resource Groups.",
       "Actions":[
          "Microsoft.Resources/subscriptions/resourceGroups/write",
          "Microsoft.Resources/subscriptions/resourceGroups/delete"
       ],
       "AssignableScopes":["/"]
    }
    EOF
    ```

3. Create a role based on the policy document:
    ```bash
    az role definition create --role-definition policy.json
    ```

4. Create a service account to access virtual machine resources as well as DNS:
    ```bash
    az ad sp create-for-rbac --name claudie-sp
    ```
    ```json
    {
      "clientId": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
      "displayName": "claudie-sp",
      "clientSecret": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
      "tenant": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
    }
    ```

5. Assign required roles for the service principal:
    ```bash
    {
      az role assignment create --assignee claudie-sp --role "Virtual Machine Contributor"
      az role assignment create --assignee claudie-sp --role "Network Contributor"
      az role assignment create --assignee claudie-sp --role "Resource Group Management"
    }
    ```

## DNS requirements
If you wish to use Azure as your DNS provider where Claudie creates DNS records pointing to Claudie managed clusters, you will need to create a **public DNS zone** by following [this guide](https://learn.microsoft.com/en-us/azure/dns/dns-getstarted-portal#prerequisites).

!!! warning "Azure is not my domain registrar"
    If you haven't acquired a domain via Azure and wish to utilize Azure for hosting your zone, you can refer to [this guide](https://learn.microsoft.com/en-us/azure/dns/dns-delegate-domain-azure-dns#retrieve-name-servers) on Azure nameservers. However, if you prefer not to use the entire domain, an alternative option is to delegate a subdomain to Azure.

## Input manifest examples
### Single provider, multi region cluster example

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
      version: v1.24.0
      network: 192.168.2.0/24
      pools:
        control:
          - control-azure
        compute:
          - compute-2-azure
          - compute-1-azure
```

### Multi provider, multi region clusters example

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
      version: v1.24.0
      network: 192.168.2.0/24
      pools:
        control:
          - control-azure-1
          - control-azure-2
        compute:
          - compute-azure-1
          - compute-azure-2
```
