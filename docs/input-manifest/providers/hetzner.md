# Hetzner
Hetzner provider requires `credentials` token field in string format, and Hetzner DNS provider requires `apitoken` field in string format.

## Compute example
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: hetzner-secret
data:
  credentials: <base64-encoded-api-token>
type: Opaque
```

!!! warning "No Load-Balanced DNS Support on Hetzner" 
    Hetzner does not support load-balanced DNS records with health checks. In the event of a virtual machine failure, the corresponding DNS A record will remain active and will not be automatically removed from the DNS database.

## Create Hetzner API credentials
You can create Hetzner API credentials by following [this guide](https://docs.hetzner.com/cloud/api/getting-started/generating-api-token/). The required permissions for the zone you want to use are:

```bash
Read & Write
```

## DNS setup
If you wish to use Hetzner as your DNS provider where Claudie creates DNS records pointing to Claudie managed clusters, you will need to create a **public DNS zone** by following [this guide](https://docs.hetzner.com/dns-console/dns/general/getting-started-dns/).

!!! warning "Hetzner is not my domain registrar"
    If you haven't acquired a domain via Hetzner and wish to utilize Hetzner for hosting your zone, you can refer to [this guide](https://docs.hetzner.com/dns-console/dns/general/dns-overview#the-hetzner-online-name-servers-are) on Hetzner nameservers. However, if you prefer not to use the entire domain, an alternative option is to delegate a subdomain to Hetzner.

## Input manifest examples

### Single provider, multi region cluster example
#### Create a secret for Hetzner provider
The secret for an Hetzner provider must include the following mandatory fields: `credentials`.

```bash
kubectl create secret generic hetzner-secret-1 --namespace=<your-namespace> --from-literal=credentials='<your-api-token>'
```

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: hetzner-example-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: hetzner-1
      providerType: hetzner
      secretRef:
        name: hetzner-secret-1
        namespace: <your-namespace>

  nodePools:
    dynamic:
      - name: control-htz
        providerSpec:
          # Name of the provider instance.
          name: hetzner-1
          # Location of the nodepool.
          region: hel1
        count: 1
        # Machine type name.
        serverType: cpx22
        # OS image name.
        image: ubuntu-24.04

      - name: compute-1-htz
        providerSpec:
          # Name of the provider instance.
          name: hetzner-1
          # Location of the nodepool.
          region: fsn1
        count: 2
        # Machine type name.
        serverType: cpx22
        # OS image name.
        image: ubuntu-24.04
        storageDiskSize: 50

      - name: compute-2-htz
        providerSpec:
          # Name of the provider instance.
          name: hetzner-1
          # Location of the nodepool.
          region: nbg1
        count: 2
        # Machine type name.
        serverType: cpx22
        # OS image name.
        image: ubuntu-24.04
        storageDiskSize: 50

  kubernetes:
    clusters:
      - name: hetzner-cluster
        version: v1.31.0
        network: 192.168.2.0/24
        pools:
          control:
            - control-htz
          compute:
            - compute-1-htz
            - compute-2-htz
```

### Multi provider, multi region clusters example
#### Create a secret for Hetzner provider
The secret for an Hetzner provider must include the following mandatory fields: `credentials`.

```bash
kubectl create secret generic hetzner-secret-1 --namespace=<your-namespace> --from-literal=credentials='<your-api-token>'
kubectl create secret generic hetzner-secret-2 --namespace=<your-namespace> --from-literal=credentials='<your-api-token>'
```

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: hetzner-example-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: hetzner-1
      providerType: hetzner
      secretRef:
        name: hetzner-secret-1
        namespace: <your-namespace>
    - name: hetzner-2
      providerType: hetzner
      secretRef:
        name: hetzner-secret-2
        namespace: <your-namespace>        

  nodePools:
    dynamic:
      - name: control-htz-1
        providerSpec:
          # Name of the provider instance.
          name: hetzner-1
          # Location of the nodepool.
          region: hel1
        count: 1
        # Machine type name.
        serverType: cpx22
        # OS image name.
        image: ubuntu-24.04

      - name: control-htz-2
        providerSpec:
          # Name of the provider instance.
          name: hetzner-2
          # Location of the nodepool.
          region: fsn1
        count: 2
        # Machine type name.
        serverType: cpx22
        # OS image name.
        image: ubuntu-24.04

      - name: compute-htz-1
        providerSpec:
          # Name of the provider instance.
          name: hetzner-1
          # Location of the nodepool.
          region: fsn1
        count: 2
        # Machine type name.
        serverType: cpx22
        # OS image name.
        image: ubuntu-24.04
        storageDiskSize: 50

      - name: compute-htz-2
        providerSpec:
          # Name of the provider instance.
          name: hetzner-2
          # Location of the nodepool.
          region: nbg1
        count: 2
        # Machine type name.
        serverType: cpx22
        # OS image name.
        image: ubuntu-24.04
        storageDiskSize: 50

  kubernetes:
    clusters:
      - name: hetzner-cluster
        version: v1.31.0
        network: 192.168.2.0/24
        pools:
          control:
            - control-htz-1
            - control-htz-2
          compute:
            - compute-htz-1
            - compute-htz-2
```
