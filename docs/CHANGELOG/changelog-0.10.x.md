# Claudie `v0.10`


!!! warning "Several major internal changes have been made to Claudie. While the `v0.10.0` version should be backward compatible with `v0.9.16`, it was not possible to test all possible cluster configuration scenarios. We advise creating backups before upgrading to v0.10.0 and later versions."

!!! note "Only v0.10.0 should be backwards compatible with the v0.9.16 version. Any other v0.10.x version will not have this guarantee."


## Most notable changes (TL;DR)

- After deploying claudie version `v0.10.0` the reconciliation loop will be initiated after the first `kubectl apply -f <your-input-manifest>` 
  that has a change in the desired state compared to the last applied version.

- Longhorn v1.9.2 will now be deployed for clusters built with claudie. For existing clusters build with `v0.9.16` manual steps need to be done 
  before deploying claudie `v0.10.0`:
    - Please read about the manual steps [here](https://longhorn.io/docs/1.9.2/deploy/upgrade/#manual-checks-before-upgrade)

- The Builder service has been completely removed from Claudie. It is also recommended that you delete the Builder deployment after deploying the v0.10.x versions of Claudie. Claudie now uses NATS instead of the builder to dispatch tasks among the other services.

- The BuilderTTL field, which was internal to Claudie's task dispatching process, was completely removed in favor of a work queue. Previously, when the BuilderTTL reached 0, a new diff with the current desired state was made, even if the scheduled task did not finish. Thus, it was possible for another task to be dispatched. This is no longer possible, as the move to NATS requires an explicit acknowledgment of the task to progress the building of the cluster.

- The identification and scheduling of tasks has been overhauled. Claudie now has an initial version of a reconciliation loop. In the v0.9.x versions of Claudie, whenever a change was detected after running `kubectl apply -f <your-input-manifest>` Claudie stopped and did not continue to health check or fix the error, even if the error was simply a network inconvenience, upon either a failure or success of building that change. As of now, with the reconciliation loop, every `kubectl apply -f <your-input-manifest>` will explicitly state the desired state of your clusters, and Claudie will try endlessly to reach that desired state. This means that, in the event of any errors, changes will be reverted and then reapplied, along with health checking, which helps identify potential misconfigurations or infrastructure issues. Claudie will then try to auto-repair these issues, if possible. The goal is to further improve the reconciliation loop with each release.

- DynamoDB was removed in favor of native locking supported by newer versions of OpenTofu which ship with Claudie `v0.10.x`

## Deployment

To deploy Claudie `v0.10.X`, please:

1. Download claudie.yaml from [release page](https://github.com/berops/claudie/releases)

2. Verify the checksum with `sha256` (optional)

   We provide checksums in `claudie_checksum.txt` you can verify the downloaded yaml files against the provided checksums.

3. Install Claudie using `kubectl`

> We strongly recommend changing the default credentials for MongoDB, MinIO before you deploy it.

```
kubectl apply -f https://github.com/berops/claudie/releases/latest/download/claudie.yaml
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


## v0.10.0


## What's Changed
- Use native state locking provided by OpenTofu instead of relying on DynamoDB [#1906](https://github.com/berops/claudie/pull/1906)

- Upgrade kubeone to v1.12.1. Claudie now supports building the following kubernetes versions: `32,33,34` [#1913](https://github.com/berops/claudie/pull/1913)

- Making use of a provider cache in the `Terraformer`, essentially removing the time spent downloading the provider on a cache hit [#1907](https://github.com/berops/claudie/pull/1907)

- Preventing kubeone from overriding `config.toml` which would collide with `NvidiaGPU` operator overrides [#1916](https://github.com/berops/claudie/pull/1916)

- Longhorn will now be deployed with the `best-effort` data-locality setting [#1933](https://github.com/berops/claudie/pull/1933)

- The `Ansibler` stage has been tweaked to take less time overall [#1917](https://github.com/berops/claudie/pull/1917)

- Genesis Cloud provider support dropped [#1941](https://github.com/berops/claudie/pull/1941)

- The `zone` field is now optional for dynamic nodepools defined in the Input Manifest. If omitted, Claudie will automatically distribute the nodes across zones [#1947](https://github.com/berops/claudie/pull/1947)

- Claudie will now deploy longhorn with version 1.9.2 [#1956](https://github.com/berops/claudie/pull/1956).
  [Manual steps](https://longhorn.io/docs/1.9.2/deploy/upgrade/#manual-checks-before-upgrade) need to be done before
  upgrading to claudie `v0.10.0` for longhorn.

- Claudie will now support GPU guest accelerator for GCP nodepools [#1952](https://github.com/berops/claudie/pull/1952) 
  Previously, it was not possible to communicate this information to the templates used to spawn the infrastructure. With
  the new changes, the GPU type and count will now be passed to the templates, correctly spawning a VM with the requested GPU.

  ```
   nodePools:
     dynamic:
       - name: gpu-workers
         providerSpec:
           name: gcp-provider
           region: europe-west1
           zone: europe-west1-b
         count: 1
         serverType: n1-standard-4
         image: ubuntu-2204-lts
         machineSpec:
           nvidiaGpuCount: 1              <-- specify number of gpus.
           nvidiaGpuType: nvidia-tesla-t4 <-- specify gpu type
  ```

- Initial version of the reconciliation loop was added to claudie [#1951](https://github.com/berops/claudie/pull/1951)
  Claudie will now endlessly healthcheck and try to fix errors on identified tasks. While currently this only resolves
  basic scenarios, as unreachable nodes, the aim is to broaden this with every release.
  
- Claudie will no longer expect NGINX to be installed on existing clusters [#1980](https://github.com/berops/claudie/pull/1980)

- Part of the reconcilation loop is to refresh the current state infrastructure periodically after no tasks has been identified [#1979](https://github.com/berops/claudie/pull/1979)

## Bug fixes

- Deletion process was fixed for newer versions of kubernetes [#1919](https://github.com/berops/claudie/pull/1919)

- Deploy `kubelet-csr-approver` to approve kubelet server CSRs [#1934](https://github.com/berops/claudie/pull/1934)
