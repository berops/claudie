# Exoscale
Exoscale cloud provider requires you to input the credentials as an `apikey` and an `apisecret`.

## Compute and DNS example
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: exoscale-secret
data:
  apikey: <base64-encoded-api-key>
  apisecret: <base64-encoded-api-secret>
type: Opaque
```

## Create Exoscale API credentials
You can create Exoscale API credentials by following [this guide](https://community.exoscale.com/documentation/iam/quick-start/). The required permissions for the API key are:

- **Compute**: full access (instances, security groups, SSH keys)
- **DNS**: full access (if using Exoscale as DNS provider)

## DNS setup
If you wish to use Exoscale as your DNS provider where Claudie creates DNS records pointing to Claudie managed clusters, you will need to create a **DNS zone** by following [this guide](https://community.exoscale.com/documentation/dns/).

!!! warning "Exoscale is not my domain registrar"
    If you haven't acquired a domain via Exoscale and wish to utilize Exoscale for hosting your zone, you will need to point your domain's nameservers to Exoscale's DNS servers (`ns1.exoscale.io`, `ns1.exoscale.com`, `ns1.exoscale.net`, `ns1.exoscale.ch`). Alternatively, you can delegate a subdomain to Exoscale.

## Available regions
Exoscale operates in the following regions:

| Region | Location |
|------|----------|
| `ch-gva-2` | Geneva, Switzerland |
| `de-fra-1` | Frankfurt, Germany |
| `de-muc-1` | Munich, Germany |
| `at-vie-1` | Vienna, Austria |
| `at-vie-2` | Vienna, Austria |
| `bg-sof-1` | Sofia, Bulgaria |

## GPU instances
Exoscale offers GPU instance types with built-in NVIDIA GPUs. GPU instances are used by setting the `serverType` to a GPU instance type (e.g., `gpu2.small`). No additional `machineSpec` configuration is needed — the GPU is included in the instance type.

!!! note "GPU availability"
    GPU instance types and availability may vary by zone and require account authorization. You can list available GPU instance types using the [Exoscale CLI](https://github.com/exoscale/cli):

    ```bash
    exo compute instance-type list --verbose | grep -i gpu
    ```

    Alternatively, check the [Exoscale instance types page](https://www.exoscale.com/pricing/#/compute) for current offerings.

## Input manifest examples

### Create a secret for Exoscale provider
The secret for an Exoscale provider must include the following mandatory fields: `apikey` and `apisecret`.

```bash
kubectl create secret generic exoscale-secret-1 --namespace=<your-namespace> --from-literal=apikey='<your-api-key>' --from-literal=apisecret='<your-api-secret>'
```

### Single-provider, multi-region cluster example

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: exoscale-example-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: exoscale-1
      providerType: exoscale
      # Exoscale templates are supported from claudie-config v0.9.18+
      templates:
        repository: "https://github.com/berops/claudie-config"
        tag: v0.9.18
        path: "templates/terraformer/exoscale"
      secretRef:
        name: exoscale-secret-1
        namespace: <your-namespace>

  nodePools:
    dynamic:
      - name: control-exo
        providerSpec:
          # Name of the provider instance.
          name: exoscale-1
          # Region of the nodepool.
          region: ch-gva-2
        count: 1
        # Instance type name.
        serverType: standard.medium
        # OS image template name.
        image: "Linux Ubuntu 24.04 LTS 64-bit"

      - name: compute-1-exo
        providerSpec:
          # Name of the provider instance.
          name: exoscale-1
          # Region of the nodepool.
          region: de-fra-1
        count: 2
        # Instance type name.
        serverType: standard.medium
        # OS image template name.
        image: "Linux Ubuntu 24.04 LTS 64-bit"
        storageDiskSize: 50

      - name: compute-2-exo
        providerSpec:
          # Name of the provider instance.
          name: exoscale-1
          # Region of the nodepool.
          region: at-vie-1
        count: 2
        # Instance type name.
        serverType: standard.medium
        # OS image template name.
        image: "Linux Ubuntu 24.04 LTS 64-bit"
        storageDiskSize: 50

  kubernetes:
    clusters:
      - name: exoscale-cluster
        version: v1.31.0
        network: 192.168.2.0/24
        pools:
          control:
            - control-exo
          compute:
            - compute-1-exo
            - compute-2-exo
```

### Multi-provider, multi-region clusters example

```bash
kubectl create secret generic exoscale-secret-1 --namespace=<your-namespace> --from-literal=apikey='<your-api-key>' --from-literal=apisecret='<your-api-secret>'
kubectl create secret generic exoscale-secret-2 --namespace=<your-namespace> --from-literal=apikey='<your-api-key>' --from-literal=apisecret='<your-api-secret>'
```

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: exoscale-example-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: exoscale-1
      providerType: exoscale
      # Exoscale templates are supported from claudie-config v0.9.18+
      templates:
        repository: "https://github.com/berops/claudie-config"
        tag: v0.9.18
        path: "templates/terraformer/exoscale"
      secretRef:
        name: exoscale-secret-1
        namespace: <your-namespace>
    - name: exoscale-2
      providerType: exoscale
      # Exoscale templates are supported from claudie-config v0.9.18+
      templates:
        repository: "https://github.com/berops/claudie-config"
        tag: v0.9.18
        path: "templates/terraformer/exoscale"
      secretRef:
        name: exoscale-secret-2
        namespace: <your-namespace>

  nodePools:
    dynamic:
      - name: control-exo-1
        providerSpec:
          # Name of the provider instance.
          name: exoscale-1
          # Region of the nodepool.
          region: ch-gva-2
        count: 1
        # Instance type name.
        serverType: standard.medium
        # OS image template name.
        image: "Linux Ubuntu 24.04 LTS 64-bit"

      - name: control-exo-2
        providerSpec:
          # Name of the provider instance.
          name: exoscale-2
          # Region of the nodepool.
          region: de-fra-1
        count: 2
        # Instance type name.
        serverType: standard.medium
        # OS image template name.
        image: "Linux Ubuntu 24.04 LTS 64-bit"

      - name: compute-exo-1
        providerSpec:
          # Name of the provider instance.
          name: exoscale-1
          # Region of the nodepool.
          region: ch-gva-2
        count: 2
        # Instance type name.
        serverType: standard.medium
        # OS image template name.
        image: "Linux Ubuntu 24.04 LTS 64-bit"
        storageDiskSize: 50

      - name: compute-exo-2
        providerSpec:
          # Name of the provider instance.
          name: exoscale-2
          # Region of the nodepool.
          region: at-vie-1
        count: 2
        # Instance type name.
        serverType: standard.medium
        # OS image template name.
        image: "Linux Ubuntu 24.04 LTS 64-bit"
        storageDiskSize: 50

  kubernetes:
    clusters:
      - name: exoscale-cluster
        version: v1.31.0
        network: 192.168.2.0/24
        pools:
          control:
            - control-exo-1
            - control-exo-2
          compute:
            - compute-exo-1
            - compute-exo-2
```