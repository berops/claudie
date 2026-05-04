# Verda Cloud
[Verda Cloud](https://verda.com/) is a Helsinki-based GPU/compute provider. The Verda provider in Claudie requires OAuth2 client credentials (`clientid` + `clientsecret`) in the Kubernetes Secret. An optional `baseurl` can override the default API endpoint.

## Compute example
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: verda-secret
data:
  clientid: <base64-encoded-client-id>
  clientsecret: <base64-encoded-client-secret>
  baseurl: <base64-encoded-base-url>  # optional, defaults to https://api.verda.com/v1
type: Opaque
```

!!! note "No DNS support"
    Verda Cloud does not provide DNS resources. If you need load balancer DNS records for Verda clusters, use a separate DNS provider (e.g., Cloudflare, AWS Route53, GCP Cloud DNS).

!!! warning "Choose a non-Docker Ubuntu image"
    Verda offers Ubuntu images both with and without Docker pre-installed (e.g., `ubuntu-24.04-cuda-12.8-open-docker` vs `ubuntu-24.04-cuda-12.8-open`). Claudie uses KubeOne for cluster bootstrap, which installs and pins its own version of `containerd.io`. Pre-installed Docker on the image conflicts with this and causes apt-get to refuse the downgrade. **Always pick a Verda image without the `-docker` suffix.** Use `ubuntu-24.04` for non-GPU nodes and `ubuntu-24.04-cuda-12.8-open` (or similar) for GPU nodes.

## Create Verda API credentials

Generate OAuth2 client credentials from the [Verda console](https://console.verda.com/) under **Keys > Cloud API Credentials**. You receive a `client_id` and a `client_secret` (the secret is shown only once at creation time).

The provider uses the OAuth2 client-credentials grant flow with scope `cloud-api-v1`. Claudie's Verda template embeds the credentials at apply time (the OpenTofu `verda` provider handles token exchange internally).

## Available locations and instance types

Locations and instance types can change. Query the live catalog with the credentials you just created:

```bash
# Get an OAuth bearer token
TOKEN=$(curl -sS -X POST https://api.verda.com/v1/oauth2/token \
  -d grant_type=client_credentials \
  -d "client_id=<your-client-id>" \
  -d "client_secret=<your-client-secret>" \
  -d scope=cloud-api-v1 | jq -r .access_token)

# List instance types (CPU, RAM, GPU per type)
curl -sS https://api.verda.com/v1/instance-types \
  -H "Authorization: Bearer $TOKEN" -H "user-agent: " | jq

# List locations
curl -sS https://api.verda.com/v1/locations \
  -H "Authorization: Bearer $TOKEN" -H "user-agent: " | jq

# List images
curl -sS https://api.verda.com/v1/images \
  -H "Authorization: Bearer $TOKEN" -H "user-agent: " | jq

# Live availability per instance type and location
curl -sS https://api.verda.com/v1/instance-availability \
  -H "Authorization: Bearer $TOKEN" -H "user-agent: " | jq
```

Common values at the time of writing:

| Field | Examples |
|---|---|
| `region` | `FIN-01`, `FIN-02`, `FIN-03` (all in Helsinki) |
| `serverType` | `CPU.4V.16G` (CPU only), `1B200.30V` (1× B200 GPU) |
| `image` | `ubuntu-24.04`, `ubuntu-24.04-cuda-12.8-open`, `ubuntu-22.04-cuda-13.0-open` |

!!! warning "Storage quota and trash"
    Verda's deleted volumes go to a per-account trash bin and continue to count toward your storage quota until permanently purged. If a Claudie apply fails after creating volumes, those volumes accumulate in the trash. Purge them with `DELETE /v1/volumes/<id>` and JSON body `{"is_permanent": true}` (HTTP 202 returned). Example: `curl -sS -X DELETE https://api.verda.com/v1/volumes/<id> -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"is_permanent": true}'`

## GPU instances
Verda specializes in GPU compute (A100, H100, B200, etc.). GPU instances are selected by setting the `serverType` to a GPU instance type (e.g., `1B200.30V` for 1× B200 with 30 vCPU). No additional `machineSpec` configuration is needed — the GPU is included in the instance type. Use the `*-cuda-*-open` (no Docker) image variants for GPU nodes; CUDA drivers are baked in.

## Custom SSH port

Like all Claudie-managed nodes, Verda VMs listen for SSH on **port 22522**. The cloud-init script reconfigures `sshd_config` and the `ssh.socket` listener accordingly during node provisioning.

## Input manifest examples

### Create a secret for Verda provider
The secret for a Verda provider must include the following mandatory fields: `clientid`, `clientsecret`. The `baseurl` field is optional.

```bash
kubectl create secret generic verda-secret-1 \
  --namespace=<your-namespace> \
  --from-literal=clientid='<your-client-id>' \
  --from-literal=clientsecret='<your-client-secret>'
```

### Single-provider, single-location cluster example

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: verda-example-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: verda-1
      providerType: verda
      templates:
        repository: "https://github.com/berops/claudie-config"
        tag: v0.11.0
        path: "templates/terraformer/verda"
      secretRef:
        name: verda-secret-1
        namespace: <your-namespace>

  nodePools:
    dynamic:
      - name: control-verda
        providerSpec:
          # Name of the provider instance.
          name: verda-1
          # Location of the nodepool.
          region: FIN-01
        count: 1
        # Instance type name.
        serverType: CPU.4V.16G
        # OS image name (use a non-Docker variant).
        image: "ubuntu-24.04"

      - name: compute-verda
        providerSpec:
          name: verda-1
          region: FIN-01
        count: 2
        serverType: CPU.4V.16G
        image: "ubuntu-24.04"
        storageDiskSize: 50

  kubernetes:
    clusters:
      - name: verda-cluster
        version: "1.34.0"
        network: 192.168.2.0/24
        pools:
          control:
            - control-verda
          compute:
            - compute-verda
```

### GPU compute pool example

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: verda-gpu-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: verda-1
      providerType: verda
      templates:
        repository: "https://github.com/berops/claudie-config"
        tag: v0.11.0
        path: "templates/terraformer/verda"
      secretRef:
        name: verda-secret-1
        namespace: <your-namespace>

  nodePools:
    dynamic:
      - name: control-verda
        providerSpec:
          name: verda-1
          region: FIN-01
        count: 1
        serverType: CPU.4V.16G
        image: "ubuntu-24.04"

      - name: gpu-verda
        providerSpec:
          name: verda-1
          region: FIN-01
        count: 1
        # GPU instance type, e.g. 1× B200.
        serverType: 1B200.30V
        # CUDA image without Docker (KubeOne installs its own containerd).
        image: "ubuntu-24.04-cuda-12.8-open"
        storageDiskSize: 100

  kubernetes:
    clusters:
      - name: verda-gpu-cluster
        version: "1.34.0"
        network: 192.168.2.0/24
        pools:
          control:
            - control-verda
          compute:
            - gpu-verda
```

### Autoscaling cluster example

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: verda-autoscale-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: verda-1
      providerType: verda
      templates:
        repository: "https://github.com/berops/claudie-config"
        tag: v0.11.0
        path: "templates/terraformer/verda"
      secretRef:
        name: verda-secret-1
        namespace: <your-namespace>

  nodePools:
    dynamic:
      - name: control-verda
        providerSpec:
          name: verda-1
          region: FIN-01
        count: 1
        serverType: CPU.4V.16G
        image: "ubuntu-24.04"

      - name: gpu-verda
        providerSpec:
          name: verda-1
          region: FIN-01
        # Autoscaler configuration (mutually exclusive with count).
        autoscaler:
          min: 0
          max: 3
        serverType: 1B200.30V
        image: "ubuntu-24.04-cuda-12.8-open"
        storageDiskSize: 100

  kubernetes:
    clusters:
      - name: verda-cluster
        version: "1.34.0"
        network: 192.168.2.0/24
        pools:
          control:
            - control-verda
          compute:
            - gpu-verda
```
