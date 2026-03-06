# CloudRift
CloudRift cloud provider requires a personal API key - `token` in the Kubernetes Secret. Optionally, a `teamid` can be provided to provision VMs under a team account.

## Compute example
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cloudrift-secret
data:
  token: <base64-encoded-api-token>
  teamid: <base64-encoded-team-id>  # optional
type: Opaque
```

!!! warning "Personal API token required"
    CloudRift SSH key operations require a **personal API key**. Team API keys will return 401 for SSH key creation. The `token` field in the Secret must be a personal API key, even if `teamid` is provided.

!!! note "No DNS support"
    CloudRift does not provide DNS resources. If you need load balancer DNS records for CloudRift clusters, use a separate DNS provider (e.g., Cloudflare, AWS Route53, GCP Cloud DNS).

## Create CloudRift API credentials
You can create a CloudRift API key from the [CloudRift console](https://console.cloudrift.ai/). Navigate to **Settings > API Keys** and generate a new personal API key.

If you want to provision VMs under a team account, retrieve the Team ID via the API:

```bash
curl -s -X POST \
  -H "X-API-KEY: <your-api-key>" \
  https://api.cloudrift.ai/api/v1/teams/list \
  | jq '.data.teams[] | {id, name}'
```

## Available datacenters and instance types

Datacenters and instance types change frequently. You can check current availability in the [CloudRift console](https://console.cloudrift.ai/) or via the API:

```bash
# List instance types with dedicated public IP, their datacenters, and pricing
curl -s -X POST \
  -H "X-API-KEY: <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{"version":"2025-06-10","data":{}}' \
  https://api.cloudrift.ai/api/v1/instance-types/list \
  | jq '.data.instance_types[] | .brand_short as $gpu | .variants[] |
      (.ip_availability_per_dc // {} | to_entries | map(select(.value.public_ips == true)) | map(.key)) as $dcs |
      select($dcs | length > 0) |
      {name: .name, gpu: $gpu, cost_per_hour: .cost_per_hour, datacenters: $dcs}'
```

```bash
# List available OS recipes (images)
curl -s -X POST \
  -H "X-API-KEY: <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{"version":"2025-06-10"}' \
  https://api.cloudrift.ai/api/v1/recipes/list \
  | jq '.data.groups[].recipes[] | select(.details.VirtualMachine != null) | {name}'
```

!!! warning "Shared public IP instances not supported"
    CloudRift offers both dedicated and shared public IP instances. Claudie requires dedicated public IP instances. Shared-IP instances use non-standard port mappings and are **not supported**. Make sure to choose an instance type that provides a dedicated public IP.

## GPU instances
CloudRift specializes in GPU compute, offering NVIDIA GPU instances (e.g., RTX 4090, RTX 5090). GPU instances are selected by setting the `serverType` to a GPU instance type (e.g., `rtx49-7-50-500-nr.1`). No additional `machineSpec` configuration is needed — the GPU is included in the instance type.

## Input manifest examples

### Create a secret for CloudRift provider
The secret for a CloudRift provider must include the following mandatory field: `token`. The `teamid` field is optional.

```bash
kubectl create secret generic cloudrift-secret-1 --namespace=<your-namespace> --from-literal=token='<your-api-token>'
```

To provision under a team account:
```bash
kubectl create secret generic cloudrift-secret-1 --namespace=<your-namespace> --from-literal=token='<your-personal-api-token>' --from-literal=teamid='<your-team-id>'
```

### Single-provider, multi-datacenter cluster example

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: cloudrift-example-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: cloudrift-1
      providerType: cloudrift
      templates:
        repository: "https://github.com/berops/claudie-config"
        tag: v0.10.0
        path: "templates/terraformer/cloudrift"
      secretRef:
        name: cloudrift-secret-1
        namespace: <your-namespace>

  nodePools:
    dynamic:
      - name: control-cr
        providerSpec:
          # Name of the provider instance.
          name: cloudrift-1
          # Datacenter of the nodepool.
          region: us-east-nc-nr-1
        count: 1
        # Instance type name.
        serverType: rtx49-7-50-500-nr.1
        # OS recipe name.
        image: "Ubuntu 22.04 Server (NVidia)"

      - name: compute-1-cr
        providerSpec:
          # Name of the provider instance.
          name: cloudrift-1
          # Datacenter of the nodepool.
          region: eu-west-uk-lo-2
        count: 2
        # Instance type name.
        serverType: rtx59-7-50-400-ec.1
        # OS recipe name.
        image: "Ubuntu 22.04 Server (NVidia)"

  kubernetes:
    clusters:
      - name: cloudrift-cluster
        version: "1.34.0"
        network: 192.168.2.0/24
        pools:
          control:
            - control-cr
          compute:
            - compute-1-cr
```

### Autoscaling cluster example

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: cloudrift-autoscale-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: cloudrift-1
      providerType: cloudrift
      templates:
        repository: "https://github.com/berops/claudie-config"
        tag: v0.10.0
        path: "templates/terraformer/cloudrift"
      secretRef:
        name: cloudrift-secret-1
        namespace: <your-namespace>

  nodePools:
    dynamic:
      - name: control-cr
        providerSpec:
          name: cloudrift-1
          region: us-east-nc-nr-1
        count: 1
        serverType: rtx49-7-50-500-nr.1
        image: "Ubuntu 22.04 Server (NVidia)"

      - name: compute-cr
        providerSpec:
          name: cloudrift-1
          region: eu-west-uk-lo-2
        # Autoscaler configuration (mutually exclusive with count).
        autoscaler:
          min: 0
          max: 3
        serverType: rtx59-7-50-400-ec.1
        image: "Ubuntu 22.04 Server (NVidia)"

  kubernetes:
    clusters:
      - name: cloudrift-cluster
        version: "1.34.0"
        network: 192.168.2.0/24
        pools:
          control:
            - control-cr
          compute:
            - compute-cr
```
