# VastAI

VastAI cloud provider requires a personal API key (`personalapikey`) and optionally accepts a team API key (`teamapikey`) in the Kubernetes Secret.

## Compute example

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: vastai-secret
data:
  personalapikey: <base64-encoded-personal-api-key>
  teamapikey: <base64-encoded-team-api-key>
type: Opaque
```

!!! warning "Personal API key required"
VastAI SSH key operations require a **personal API key**; team API keys are not supported for them. If a team API key is provided, VMs are provisioned under the team account, and SSH keys created under the personal account are automatically loaded onto them.

!!! note "No DNS support"
VastAI does not provide DNS resources. If you need load balancer DNS records for VastAI clusters, use a separate DNS provider (e.g., Cloudflare, AWS Route53, GCP Cloud DNS).

## Create VastAI API credentials

You can create a VastAI API key in the [VastAI console](https://cloud.vast.ai/). In the left sidebar, select **Keys**, then **API Keys**, and generate a new personal API key.

To create a team API key, click your account name at the top of the left sidebar and open **API Keys** again. Enable **Team View** using the toggle in the top-right corner and generate a new key.

## Available machines

VastAI is a marketplace, so unlike other providers it has no stable instance types. Resources are identified by three IDs:

- `MachineId` - unique ID of a physical machine for its entire lifetime.
- `OfferId` - ID of a specific offer on a machine. It is removed once the offer is rented.
- `InstanceId` - ID of a rented instance.

The VastAI API allows renting only by `OfferId`. Instead of referencing an instance type, you specify the machine parameters in the nodepool and Claudie searches for a matching offer via the [search offers](https://docs.vast.ai/api-reference/search/search-offers) endpoint.

### Default search parameters

The search offers endpoint supports many more parameters than the InputManifest exposes. Claudie always applies the following ones:

| VastAI API param | Value                 | Description                                        |
| ---------------- | --------------------- | -------------------------------------------------- |
| `type`           | `ondemand`            | On-demand instances only (no interruptible ones).  |
| `verified`       | `true`                | Verified machines only.                            |
| `reliability`    | `>= 0.98`             | Minimum machine reliability score.                 |
| `vms_enabled`    | `true`                | Machines supporting VM instances.                  |
| `static_ip`      | `true`                | Machines with a static IP address.                 |
| `duration`       | `>= 2592000`          | Offer available for at least 30 days (in seconds). |
| `inet_down`      | `>= 300`              | Minimum download bandwidth in Mbps.                |
| `order`          | `dph_total` ascending | Sort the offers from the cheapest one.             |

### Machine parameters

The following InputManifest fields map to VastAI API parameters:

| InputManifest field          | VastAI API param | VastAI endpoint                                                                 |
| ---------------------------- | ---------------- | ------------------------------------------------------------------------------- |
| `serverType`                 | `cpu_arch`       | [search offers](https://docs.vast.ai/api-reference/search/search-offers)        |
| `region`                     | `geolocation`    | [search offers](https://docs.vast.ai/api-reference/search/search-offers)        |
| `storageDiskSize`            | `disk_space`     | [search offers](https://docs.vast.ai/api-reference/search/search-offers)        |
| `storageDiskSize`            | `disk`           | [create instance](https://docs.vast.ai/api-reference/instances/create-instance) |
| `machineSpec.cpuCount`       | `cpu_cores`      | [search offers](https://docs.vast.ai/api-reference/search/search-offers)        |
| `machineSpec.memory`         | `gpu_total_ram`  | [search offers](https://docs.vast.ai/api-reference/search/search-offers)        |
| `machineSpec.nvidiaGpuType`  | `gpu_name`       | [search offers](https://docs.vast.ai/api-reference/search/search-offers)        |
| `machineSpec.nvidiaGpuCount` | `num_gpus`       | [search offers](https://docs.vast.ai/api-reference/search/search-offers)        |
| `image`                      | `image`          | [create instance](https://docs.vast.ai/api-reference/instances/create-instance) |

!!! note "Machine spec values are minimums"
`machineSpec.cpuCount`, `machineSpec.memory` and `storageDiskSize` set minimum requirements, so offers with more CPU cores, memory or disk space also match. `machineSpec.memory` is the total **GPU** RAM (in MB), not the system RAM. The Ubuntu VM image needs at least 130 GB of storage, so set `storageDiskSize` to 130 or more.

The VastAI location filter accepts only two-letter country codes. To simplify this, the `region` field accepts either:

- a space-separated list of country codes, e.g. `region: DE US GB PL SK`. The list can be at most 63 characters long, because Claudie also uses the region as a Kubernetes node label value.
- one of the predefined regions: `europe`, `asia`, `africa`, `north-america`, `south-america`, `oceania` (see the [region mapping](#region-mapping) below)

!!! note "Pricing"
The price of a node is not fixed, it depends on the offer that is rented. Claudie sorts the matching offers by price and rents the cheapest one. If that offer is no longer available, it tries the next one in the list.

## Input manifest examples

### Create a secret for VastAI provider

The secret for a VastAI provider must include `personalapikey` and can optionally include `teamapikey`.

```bash
kubectl create secret generic vastai-secret-1 --namespace=<your-namespace> --from-literal=personalapikey='<your-personal-api-key>' --from-literal=teamapikey='<your-team-api-key>'
```

### Single provider cluster example

VastAI nodepools can only be used as compute (worker) nodepools. Renting GPU instances for control-plane or load balancer nodes is not intended for this provider, so Claudie rejects any input manifest that references a VastAI nodepool in a `control` pool or a load balancer `pools` list. This example therefore uses the Hetzner provider for the control node.

```yaml
apiVersion: claudie.io/v1beta1
kind: TemplateGitReference
metadata:
  name: vastai-templates
  namespace: claudie
spec:
  endpoint:
    url: github.com/berops/claudie-config
    protocol: https
  commit: release
  paths:
    terraformer: templates/terraformer
    playbooks: templates/playbooks
    configLb: templates/config-lb
    configK8s: templates/config-k8s
    manifestsK8s: templates/manifests-k8s
---
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: vastai-example-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: vastai-1
      providerType: vastai
      templatesRef:
        name: vastai-templates
        namespace: claudie
      secretRef:
        name: vastai-secret-1
        namespace: <your-namespace>
    - name: hetzner-1
      providerType: hetzner
      secretRef:
        name: hetzner-secret-1
        namespace: <your-namespace>

  nodePools:
    dynamic:
      - name: control-htz
        # VastAI does not support control nodes, Hetzner is used as an example.
        providerSpec:
          name: hetzner-1
          region: hel1
        count: 1
        serverType: cpx22
        image: ubuntu-24.04
      - name: compute-vastai
        providerSpec:
          name: vastai-1
          # Predefined region or a space-separated list of country codes.
          region: europe
        count: 1
        # CPU architecture.
        serverType: amd64
        image: docker.io/vastai/kvm:@vastai-automatic-tag
        storageDiskSize: 150
        machineSpec:
          cpuCount: 1
          memory: 4096
          nvidiaGpuCount: 1
          nvidiaGpuType: RTX 4090

  kubernetes:
    clusters:
      - name: vastai-cluster
        version: "1.35.0"
        network: 192.168.2.0/24
        pools:
          control:
            - control-htz
          compute:
            - compute-vastai
```

## Region mapping

| Region          | Country codes                                                                                                                                                                 |
| --------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `europe`        | AD AL AT AX BA BE BG BY CH CY CZ DE DK EE ES FI FO FR GB GG GI GR HR HU IE IM IS IT JE LI LT LU LV MC MD ME MK MT NL NO PL PT RO RS SE SI SJ SK SM UA VA XK                   |
| `asia`          | AE AF AM AZ BD BH BN BT CN GE HK ID IL IN IO IQ IR JO JP KG KH KP KR KW KZ LA LB LK MM MN MO MV MY NP OM PH PK PS QA RU SA SG SY TH TJ TL TM TR TW UZ VN YE                   |
| `africa`        | AO BF BI BJ BW CD CF CG CI CM CV DJ DZ EG EH ER ET GA GH GM GN GQ GW KE KM LR LS LY MA MG ML MR MU MW MZ NA NE NG RE RW SC SD SH SL SN SO SS ST SZ TD TG TN TZ UG YT ZA ZM ZW |
| `north-america` | AG AI AW BB BL BM BQ BS BZ CA CR CU CW DM DO GD GL GP GT HN HT JM KN KY LC MF MQ MS MX NI PA PM PR SV SX TC TT US VC VG VI                                                    |
| `south-america` | AR BO BR CL CO EC FK GF GY PE PY SR UY VE                                                                                                                                     |
| `oceania`       | AS AU CC CK CX FJ FM GU KI MH MP NC NF NR NU NZ PF PG PN PW SB TK TO TV UM VU WF WS                                                                                           |
