# Claudie `v0.16`

!!! warning "This release is not backwards compatible with previous `v0.15.x` versions, due to an internal rework of handling state files for clusters"

## Readme Before Deploying 

## ⚠️ Breaking Change: No upgrade path from pre-`v0.16.x` clusters

Clusters created with Claudie versions older than `v0.16.x` are **not compatible** with this release and cannot be upgraded in place.

### Migration steps

1. Back up all workloads running on your existing cluster.
2. Using your **current (old) Claudie version**, destroy the existing cluster. `v0.16.x` cannot destroy clusters created by older versions, so this step must be completed before upgrading.
3. Upgrade Claudie to `v0.16.x`.
4. Create a new cluster with `v0.16.x`.
5. Redeploy your workloads to the new cluster.

> **Note:** Do not upgrade Claudie before destroying the old cluster. Once you are on `v0.16.x`, the old cluster can no longer be torn down through Claudie.

## Deployment

To deploy Claudie `v0.16.x`, please:

1. Download Claudie.yaml from [release page](https://github.com/berops/claudie/releases)

2. Verify the checksum with `sha256` (optional)

   We provide checksums in `claudie_checksum.txt` you can verify the downloaded yaml files against the provided checksums.

3. Install Claudie using `kubectl`

> We strongly recommend changing the default credentials for MongoDB, MinIO before you deploy it.

```
kubectl apply -f https://github.com/berops/claudie/releases/latest/download/Claudie.yaml
```

To further harden Claudie, you may want to deploy our pre-defined network policies:
   ```bash
   # for clusters using cilium as their CNI
   kubectl apply -f https://github.com/berops/claudie/releases/latest/download/network-policy-cilium.yaml
   ```
   ```bash
   # other
   kubectl apply -f https://github.com/berops/claudie/releases/latest/download/network-policy.yaml
   ```

## What's Changed
- External templates handling has been reworked [#2165](https://github.com/berops/claudie/pull/2165)

  External templates are now defined in their own CRD called `TemplateGitReference` instead of directly
  defining them inside the `InputManifest`. You can read further about the new CRD in our documentation.

An example custom resource is provided below:

```yaml
apiVersion: claudie.io/v1beta1
kind: TemplateGitReference
metadata:
  name: my-templates
  namespace: claudie
spec:
  endpoint:
    url: github.com/berops/claudie-config
    protocol: https
  auth:
    secretRef:
      name: git-token
      namespace: claudie
  commit: latest
  paths:
    terraformer: templates/terraformer
    playbooks: templates/playbooks
    configLb: templates/config-lb
    configK8s: templates/config-k8s
    manifestsK8s: templates/manifests-k8s 
```

Inside the `InputManifest` you can then reference the Custom Resource under the provider as follows:

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: example
spec:
  providers:
    - name: hetzner-1
      providerType: hetzner
      templatesRef:
        name: example-templates
        namespace: claudie
...
```

If no templates are specified Claudie generates a default custom resource that points to the configuration in `https://www.github.com/berops/claudie-config`.

- Update kubeone. The currently supported kubernetes versions are `1.34.x, 1.35.x, and 1.36.x.` [#2184](https://github.com/berops/claudie/pull/2184)

- Upgrade longhorn to v1.12 [#2188](https://github.com/berops/claudie/pull/2188)

- Update tofu to 1.12.5 [#2186](https://github.com/berops/claudie/pull/2186)

- Updated generated kubeconfig secret name to omit auto-generated hash [#2183](https://github.com/berops/claudie/pull/2183)

- Dropped raw ICMP pings in favor of SSH pings [#2191](https://github.com/berops/claudie/pull/2191)

- A single state file per cluster has been dropped in favor of a single state file per nodepool with an additional shared infrastructure state file for each cluster [#2177](https://github.com/berops/claudie/pull/2177).
  This speeds up scaling up/scaling down existing nodepools.

- General maintainence dependencies update [#2201](https://github.com/berops/claudie/pull/2201)

## Bug fixes

- fix dangling generated directory [#2189](https://github.com/berops/claudie/pull/2189)

- fix unreachable node deletion pipeline [#2193](https://github.com/berops/claudie/pull/2193)

- fix leaked containerd-task #[2203](https://github.com/berops/claudie/pull/2203)
