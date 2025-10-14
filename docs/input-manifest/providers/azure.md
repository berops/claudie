# Azure
Azure provider requires you to input `clientsecret`, `subscriptionid`, `tenantid`, and `clientid`.

## Compute and DNS example
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: azure-secret
data:
  clientid: QWJjZH5FRmd+SDZJamtsc35BQkMxNXNFRkdLNTRzNzhYfk9sazk=
  # all resources you define will be charged here
  clientsecret: NmE0ZGZzZzctc2Q0di1mNGFkLWRzdmEtYWQ0djYxNmZkNTEy
  subscriptionid: NTRjZGFmYTUtc2R2cy00NWRzLTU0NnMtZGY2NTFzZmR0NjE0
  tenantid: MDI1NXNjMjMtNzZ3ZS04N2c2LTk2NGYtYWJjMWRlZjJnaDNs
type: Opaque

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

!!! warning "To create custom roles, your organization needs Microsoft Entra ID Premium P1 or P2."
    If you do not have Premium P1 or P2 activated, you can use the built-in role **Kubernetes Agent Subscription Level Operator** instead, which includes the required resource group permissions.

3. Create a role based on the policy document. Skip this step if using build in role **Kubernetes Agent Subscription Level Operator**:
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
      az role assignment create --assignee claudie-sp --role "Virtual Machine Contributor" --scope /subscriptions/<subscription_id>
      az role assignment create --assignee claudie-sp --role "Network Contributor" --scope --scope /subscriptions/<subscription_id>
      az role assignment create --assignee claudie-sp --role "Resource Group Management" --scope --scope /subscriptions/<subscription_id>
    }
    ```

!!! warning "Use built-in role as alternative to custom role"
    If you're not using the custom **Resource Group Management** role, assign the built-in role **Kubernetes Agent Subscription Level Operator**.

## DNS requirements
If you wish to use Azure as your DNS provider where Claudie creates DNS records pointing to Claudie managed clusters, you will need to create a **public DNS zone** by following [this guide](https://learn.microsoft.com/en-us/azure/dns/dns-getstarted-portal#prerequisites).

!!! warning "Azure is not my domain registrar"
    If you haven't acquired a domain via Azure and wish to utilize Azure for hosting your zone, you can refer to [this guide](https://learn.microsoft.com/en-us/azure/dns/dns-delegate-domain-azure-dns#retrieve-name-servers) on Azure nameservers. However, if you prefer not to use the entire domain, an alternative option is to delegate a subdomain to Azure.

## Input manifest examples
### Single provider, multi region cluster example

#### Create a secret for Azure provider
The secret for an Azure provider must include the following mandatory fields: `clientsecret`, `subscriptionid`, `tenantid`, and `clientid`.

```bash
kubectl create secret generic azure-secret-1 --namespace=mynamespace --from-literal=clientsecret='Abcd~EFg~H6Ijkls~ABC15sEFGK54s78X~Olk9' --from-literal=subscriptionid='6a4dfsg7-sd4v-f4ad-dsva-ad4v616fd512' --from-literal=tenantid='54cdafa5-sdvs-45ds-546s-df651sfdt614' --from-literal=clientid='0255sc23-76we-87g6-964f-abc1def2gh3l'
```

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: azure-example-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: azure-1
      providerType: azure
      secretRef:
        name: azure-secret-1
        namespace: mynamespace
  nodePools:
    dynamic:
      - name: control-az
        providerSpec:
          # Name of the provider instance.
          name: azure-1
          # Location of the nodepool.
          region: North Europe
          # Zone of the nodepool.
          zone: "1"
        count: 2
        # VM size name.
        serverType: Standard_B2s
        # URN of the image.
        image: Canonical:ubuntu-24_04-lts:server:24.04.202510010

      - name: compute-1-az
        providerSpec:
          # Name of the provider instance.
          name: azure-1
          # Location of the nodepool.
          region: Germany West Central
          # Zone of the nodepool.
          zone: "1"
        count: 2
        # VM size name.
        serverType: Standard_B2s
        # URN of the image.
        image: Canonical:ubuntu-24_04-lts:server:24.04.202510010
        storageDiskSize: 50

      - name: compute-2-az
        providerSpec:
          # Name of the provider instance.
          name: azure-1
          # Location of the nodepool.
          region: North Europe
          # Zone of the nodepool.
          zone: "1"
        count: 2
        # VM size name.
        serverType: Standard_B2s
        # URN of the image.
        image: Canonical:ubuntu-24_04-lts:server:24.04.202510010
        storageDiskSize: 50

  kubernetes:
    clusters:
      - name: azure-cluster
        version: v1.31.0
        network: 192.168.2.0/24
        pools:
          control:
            - control-az
          compute:
            - compute-2-az
            - compute-1-az
```

### Multi provider, multi region clusters example

```bash
kubectl create secret generic azure-secret-1 --namespace=mynamespace --from-literal=clientsecret='Abcd~EFg~H6Ijkls~ABC15sEFGK54s78X~Olk9' --from-literal=subscriptionid='6a4dfsg7-sd4v-f4ad-dsva-ad4v616fd512' --from-literal=tenantid='54cdafa5-sdvs-45ds-546s-df651sfdt614' --from-literal=clientid='0255sc23-76we-87g6-964f-abc1def2gh3l'

kubectl create secret generic azure-secret-2 --namespace=mynamespace --from-literal=clientsecret='Efgh~ijkL~on43noi~NiuscviBUIds78X~UkL7' --from-literal=subscriptionid='0965bd5b-usa3-as3c-ads1-csdaba6fd512' --from-literal=tenantid='55safa5d-dsfg-546s-45ds-d51251sfdaba' --from-literal=clientid='076wsc23-sdv2-09cA-8sd9-oigv23npn1p2'
```

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: azure-example-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: azure-1
      providerType: azure
      secretRef:
        name: azure-secret-1
        namespace: mynamespace

    - name: azure-2
      providerType: azure
      secretRef:
        name: azure-secret-2
        namespace: mynamespace

  nodePools:
    dynamic:
      - name: control-az-1
        providerSpec:
          # Name of the provider instance.
          name: azure-1
          # Location of the nodepool.
          region: North Europe
          # Zone of the nodepool.
          zone: "1"
        count: 1
        # VM size name.
        serverType: Standard_B2s
        # URN of the image.
        image: Canonical:ubuntu-24_04-lts:server:24.04.202510010

      - name: control-az-2
        providerSpec:
          # Name of the provider instance.
          name: azure-2
          # Location of the nodepool.
          region: Germany West Central
          # Zone of the nodepool.
          zone: "2"
        count: 2
        # VM size name.
        serverType: Standard_B2s
        # URN of the image.
        image: Canonical:ubuntu-24_04-lts:server:24.04.202510010

      - name: compute-az-1
        providerSpec:
          # Name of the provider instance.
          name: azure-1
          # Location of the nodepool.
          region: Germany West Central
          # Zone of the nodepool.
          zone: "2"
        count: 2
        # VM size name.
        serverType: Standard_B2s
        # URN of the image.
        image: Canonical:ubuntu-24_04-lts:server:24.04.202510010
        storageDiskSize: 50

      - name: compute-az-2
        providerSpec:
          # Name of the provider instance.
          name: azure-2
          # Location of the nodepool.
          region: North Europe
          # Zone of the nodepool.
          zone: "1"
        count: 2
        # VM size name.
        serverType: Standard_B2s
        # URN of the image.
        image: Canonical:ubuntu-24_04-lts:server:24.04.202510010
        storageDiskSize: 50

  kubernetes:
    clusters:
      - name: azure-cluster
        version: v1.31.0
        network: 192.168.2.0/24
        pools:
          control:
            - control-az-1
            - control-az-2
          compute:
            - compute-az-1
            - compute-az-2
```
