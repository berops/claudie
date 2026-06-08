# OVHcloud
[OVHcloud](https://www.ovhcloud.com/) is a European cloud provider with data centres in France, Germany, the UK, Poland, Canada and other locations. Claudie's OVH provider uses the native `ovh/ovh` OpenTofu provider with OAuth2 client credentials. It supports both compute and DNS, so an OVH provider entry can be reused as a load-balancer DNS provider.

## Compute example

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ovh-secret
data:
  clientid: <base64-encoded-client-id>
  clientsecret: <base64-encoded-client-secret>
  servicename: <base64-encoded-public-cloud-project-id>
  endpoint: <base64-encoded-endpoint>  # optional, defaults to ovh-eu
type: Opaque
```

!!! note "No native security groups"
    The `ovh/ovh` provider does not expose security group or firewall resources. Claudie configures host-level `iptables` rules in cloud-init (the same approach as the CloudRift provider). KubeOne disables UFW during node bootstrap, but the iptables rules persist via `iptables-persistent`.

!!! note "Public IP by default"
    Every Claudie-provisioned OVH instance gets a routable public IPv4 (`network { public = true }`). Floating IPs are not modelled in v1 because the OVH provider does not yet expose a native floating-IP resource ([issue](https://github.com/ovh/terraform-provider-ovh/issues/1245)).

## Create OVH API credentials

Claudie uses the OAuth2 client-credentials grant. Follow the [OVH provider OAuth2 setup guide](https://search.opentofu.org/provider/ovh/ovh/latest#oauth2) to create a service account and obtain the `client_id` and `client_secret`. Scope the service account for your Public Cloud project, and for your DNS zone if you plan to use OVH as a DNS provider for load balancers.

The Public Cloud **project ID** (the `servicename` field in the Claudie Secret) is in the OVHcloud Manager under **Public Cloud -> Project Management**.

The `endpoint` field selects which OVHcloud API region to authenticate against (not the public-cloud region for instances). Valid values: `ovh-eu` (default), `ovh-us`, `ovh-ca`, `kimsufi-eu`, `kimsufi-ca`, `soyoustart-eu`, `soyoustart-ca`.

## Available regions and flavors

Public Cloud regions are listed at <https://www.ovhcloud.com/en/about-us/global-infrastructure/regions/>.

Flavors and images vary per region. List the live catalog for a region with the `ovhcloud` CLI or the API:

```bash
# Using ovhcloud CLI (https://github.com/ovh/ovhcloud-cli)
ovhcloud cloud project flavor list --service-name <project-id> --region GRA11
ovhcloud cloud project image list  --service-name <project-id> --region GRA11

# Or directly via the API (substitute your credentials)
curl -sS "https://eu.api.ovh.com/v1/cloud/project/<project-id>/flavor?region=GRA11" \
  -H "X-Ovh-Application: <app-key>" \
  -H "X-Ovh-Consumer: <consumer-key>" | jq
```

Common flavor families:

| Family | Use case |
|--------|----------|
| `b3-*`, `b3-*-flex` | General-purpose balanced compute |
| `c3-*` | CPU-optimised |
| `r3-*` | RAM-optimised |
| `d2-*` | Discovery (cheap, low spec) |
| `i2-*` | Storage-optimised (NVMe) |
| `t1-*`, `t2-*`, `t3-*` | GPU (NVIDIA) |

## GPU instances

OVH GPU flavors are listed via the same flavor API as non-GPU flavors. A static GPU nodepool needs only `serverType` set to a GPU flavor UUID (e.g. `t2-45`, `t1-le-2`):

```yaml
- name: gpu-ovh
  providerSpec:
    name: ovh-1
    region: GRA11
  count: 1
  serverType: t1-le-2
  image: "Ubuntu 24.04"
```

For an **autoscaled** GPU nodepool (`autoscaler.min: 0`), also set `machineSpec.nvidiaGpuCount` so the cluster-autoscaler can size the node when scaling from zero. The OVH flavor API does not expose per-flavor GPU counts; without this field the autoscaler assumes zero GPUs and will not scale up to satisfy GPU-requesting pods.

After the cluster is built, install the NVIDIA GPU operator separately (Claudie does not deploy it). The operator brings up drivers, the container toolkit, and the device plugin so `nvidia.com/gpu` becomes allocatable on the node.

Verify GPU availability in the chosen region with `ovhcloud cloud project flavor list` before applying.

## Custom SSH port

Like all Claudie-managed nodes, OVH VMs listen for SSH on **port 22522**. The cloud-init script reconfigures `sshd_config` and the `ssh.socket` listener during node provisioning. Inbound 22522/tcp is opened by the iptables firewall script.

## Block storage

`storageDiskSize` is currently a no-op for OVH nodepools: data (including Longhorn's `/opt/claudie/data`) lives on the OS disk. OVH Public Cloud supports block volumes, but the upstream [`ovh/ovh` Terraform provider](https://registry.terraform.io/providers/ovh/ovh) exposes only `ovh_cloud_project_volume` (create) and offers no resource to attach an existing volume to an instance, so Claudie cannot wire it through declaratively. Support will be added once the provider gains a `volume_attach`-style resource. Until then, attach extra volumes manually via the OVHcloud Manager if you need additional capacity.

## Input manifest examples

### Create a Secret for the OVH provider

```bash
kubectl create secret generic ovh-secret-1 \
  --namespace=<your-namespace> \
  --from-literal=clientid='<your-client-id>' \
  --from-literal=clientsecret='<your-client-secret>' \
  --from-literal=servicename='<your-public-cloud-project-id>'
# Optionally add: --from-literal=endpoint='ovh-eu'
```

### Single-provider, single-region cluster

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: ovh-example-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: ovh-1
      providerType: ovh
      templates:
        repository: "https://github.com/berops/claudie-config"
        tag: v0.11.2
        path: "templates/terraformer/ovh"
      secretRef:
        name: ovh-secret-1
        namespace: <your-namespace>

  nodePools:
    dynamic:
      - name: control-ovh
        providerSpec:
          name: ovh-1
          region: GRA11
        count: 1
        serverType: b3-8
        image: "Ubuntu 24.04"

      - name: compute-ovh
        providerSpec:
          name: ovh-1
          region: GRA11
        count: 2
        serverType: b3-8
        image: "Ubuntu 24.04"

  kubernetes:
    clusters:
      - name: ovh-cluster
        version: "1.34.0"
        network: 192.168.2.0/24
        pools:
          control:
            - control-ovh
          compute:
            - compute-ovh
```

### GPU compute pool

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: ovh-gpu-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: ovh-1
      providerType: ovh
      templates:
        repository: "https://github.com/berops/claudie-config"
        tag: v0.11.2
        path: "templates/terraformer/ovh"
      secretRef:
        name: ovh-secret-1
        namespace: <your-namespace>

  nodePools:
    dynamic:
      - name: control-ovh
        providerSpec:
          name: ovh-1
          region: GRA11
        count: 1
        serverType: b3-8
        image: "Ubuntu 24.04"

      - name: gpu-ovh
        providerSpec:
          name: ovh-1
          region: GRA11
        count: 1
        serverType: t1-le-2
        image: "Ubuntu 24.04"

  kubernetes:
    clusters:
      - name: ovh-gpu-cluster
        version: "1.34.0"
        network: 192.168.2.0/24
        pools:
          control:
            - control-ovh
          compute:
            - gpu-ovh
```

### Cluster with load balancer using OVH DNS

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: ovh-lb-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: ovh-1
      providerType: ovh
      templates:
        repository: "https://github.com/berops/claudie-config"
        tag: v0.11.2
        path: "templates/terraformer/ovh"
      secretRef:
        name: ovh-secret-1
        namespace: <your-namespace>

  nodePools:
    dynamic:
      - name: control-ovh
        providerSpec:
          name: ovh-1
          region: GRA11
        count: 1
        serverType: b3-8
        image: "Ubuntu 24.04"

      - name: compute-ovh
        providerSpec:
          name: ovh-1
          region: GRA11
        count: 2
        serverType: b3-8
        image: "Ubuntu 24.04"

      - name: lb-ovh
        providerSpec:
          name: ovh-1
          region: GRA11
        count: 1
        serverType: b3-8
        image: "Ubuntu 24.04"

  kubernetes:
    clusters:
      - name: ovh-cluster
        version: "1.34.0"
        network: 192.168.2.0/24
        pools:
          control:
            - control-ovh
          compute:
            - compute-ovh

  loadBalancers:
    roles:
      - name: ingress
        protocol: tcp
        port: 80
        targetPort: 30080
        targetPools:
          - compute-ovh
    clusters:
      - name: ovh-lb
        roles:
          - ingress
        dns:
          dnsZone: example.com
          provider: ovh-1
        targetedK8s: ovh-cluster
        pools:
          - lb-ovh
```

### Autoscaling compute pool

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: ovh-autoscale-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: ovh-1
      providerType: ovh
      templates:
        repository: "https://github.com/berops/claudie-config"
        tag: v0.11.2
        path: "templates/terraformer/ovh"
      secretRef:
        name: ovh-secret-1
        namespace: <your-namespace>

  nodePools:
    dynamic:
      - name: control-ovh
        providerSpec:
          name: ovh-1
          region: GRA11
        count: 1
        serverType: b3-8
        image: "Ubuntu 24.04"

      - name: compute-ovh
        providerSpec:
          name: ovh-1
          region: GRA11
        autoscaler:
          min: 1
          max: 3
        serverType: b3-8
        image: "Ubuntu 24.04"

  kubernetes:
    clusters:
      - name: ovh-cluster
        version: "1.34.0"
        network: 192.168.2.0/24
        pools:
          control:
            - control-ovh
          compute:
            - compute-ovh
```
