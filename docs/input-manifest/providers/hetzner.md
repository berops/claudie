# Hetzner
Hetzner provider requires `credentials` token field in string format, and Hetzner DNS provider requires `apitoken` field in string format.

## Compute example
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: hetzner-secret
data:
  credentials: a3NsSVNBODc4YTZldFlBZlhZY2c1aVl5ckZHTmxDeGNJQ28wNjBIVkV5Z2pGczIxbnNrZTc2a3NqS2tvMjFscA==
type: Opaque

```

## DNS example
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: hetznerdns-secret
data:
  apitoken: a1V0UmcxcGdqQ1JhYXBQbWQ3cEFJalZnaHVyWG8xY24=
type: Opaque
```

## Create Hetzner API credentials
You can create Hetzner API credentials by following [this guide](https://docs.hetzner.com/cloud/api/getting-started/generating-api-token/). The required permissions for the zone you want to use are:

```bash
Read & Write
```

## Create Hetzner DNS credentials
You can create Hetzner DNS credentials by following [this guide](https://docs.hetzner.com/dns-console/dns/general/api-access-token/).

!!! note "DNS provider specification"
    The provider for DNS is different from the one for the Cloud.

## DNS setup
If you wish to use Hetzner as your DNS provider where Claudie creates DNS records pointing to Claudie managed clusters, you will need to create a **public DNS zone** by following [this guide](https://docs.hetzner.com/dns-console/dns/general/getting-started-dns/).

!!! warning "Hetzner is not my domain registrar"
    If you haven't acquired a domain via Hetzner and wish to utilize Hetzner for hosting your zone, you can refer to [this guide](https://docs.hetzner.com/dns-console/dns/general/dns-overview#the-hetzner-online-name-servers-are) on Hetzner nameservers. However, if you prefer not to use the entire domain, an alternative option is to delegate a subdomain to Hetzner.

## Input manifest examples

### Single provider, multi region cluster example
#### Create a secret for Hetzner provider
The secret for an Azure provider must include the following mandatory fields: `credentials`.

```bash
kubectl create secret generic hetzner-secret-1 --namespace=mynamespace --from-literal=credentials='kslISA878a6etYAfXYcg5iYyrFGNlCxcICo060HVEygjFs21nske76ksjKko21lp'
```

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: HetznerExampleManifest
spec:
  providers:
    - name: hetzner-1
      providerType: hetzner
      secretRef:
        name: hetzner-secret-1
        namespace: mynamespace

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
        # OS image name.
        image: ubuntu-22.04

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
        # OS image name.
        image: ubuntu-22.04
        storageDiskSize: 50

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
        # OS image name.
        image: ubuntu-22.04
        storageDiskSize: 50

  kubernetes:
    clusters:
      - name: hetzner-cluster
        version: v1.24.0
        network: 192.168.2.0/24
        pools:
          control:
            - control-hetzner
          compute:
            - compute-1-hetzner
            - compute-2-hetzner
```

### Multi provider, multi region clusters example
#### Create a secret for Hetzner provider
The secret for an Azure provider must include the following mandatory fields: `credentials`.

```bash
kubectl create secret generic hetzner-secret-1 --namespace=mynamespace --from-literal=credentials='kslISA878a6etYAfXYcg5iYyrFGNlCxcICo060HVEygjFs21nske76ksjKko21lp'
kubectl create secret generic hetzner-secret-2 --namespace=mynamespace --from-literal=credentials='kslIIOUYBiuui7iGBYIUiuybpiUB87bgPyuCo060HVEygjFs21nske76ksjKko21l'
```

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: HetznerExampleManifest
spec:
  providers:
    - name: hetzner-1
      providerType: hetzner
      secretRef:
        name: hetzner-secret-1
        namespace: mynamespace
    - name: hetzner-2
      providerType: hetzner
      secretRef:
        name: hetzner-secret-2
        namespace: mynamespace        

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
      # OS image name.
      image: ubuntu-22.04

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
      # OS image name.
      image: ubuntu-22.04

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
      # OS image name.
      image: ubuntu-22.04
      storageDiskSize: 50

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
      # OS image name.
      image: ubuntu-22.04
      storageDiskSize: 50

kubernetes:
  clusters:
    - name: hetzner-cluster
      version: v1.24.0
      network: 192.168.2.0/24
      pools:
        control:
          - control-hetzner-1
          - control-hetzner-2
        compute:
          - compute-hetzner-1
          - compute-hetzner-2
```
