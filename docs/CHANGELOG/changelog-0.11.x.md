# Claudie `v0.11`

!!! warning "Upgrade to this release from previous version requires manual intervention (due to the Longhorn version upgrade)."

## Deployment

To deploy Claudie `v0.11.X`, please:

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


## v0.11.0

## What's Changed
- Added [CloudRift](https://www.cloudrift.ai/) cloud provider support [#2000](https://github.com/berops/claudie/pull/2000)
  
- Updated longhorn to version v1.11.1 [#2007](https://github.com/berops/claudie/pull/2007)   
  Before upgrading to this Claudie version from v0.10.2, detach all Longhorn volumes and follow the manual checks described here: <https://longhorn.io/docs/1.11.1/deploy/upgrade/#manual-checks-before-upgrade>

- More validation of the input manifest was moved into the webhook for the operator so that more immediate feedback is given when `kubectl apply` is executed [#2008](https://github.com/berops/claudie/pull/2008)
  
- When a node is scheduled for deletion, its drain is now limited to a ~30 minute timeout, after which the node will be deleted  [#2011](https://github.com/berops/claudie/pull/2011)

- For node deletion disk scheduling on the longhorn level will now be applied before the node is deleted [#2012](https://github.com/berops/claudie/pull/2012)

## v0.11.1

## What's Changed
- General maintenance update by updating dependencies. [`#2020`](https://github.com/berops/claudie/pull/2020)

## v0.11.2

## What's Changed
- Add custom SSH port support for dynamic and static nodepools by [#2026](https://github.com/berops/claudie/pull/2026)
  
  The requirement of the SSH port to be opened at `22` has been dropped. It is now possible for external templates to define
  their own SSH port to which Claudie will connect to. The same applies to static nodepools which have the option exposed in the InputManifest
  
```
static:
  - name: control
    sshPort: 2222  # Optional: SSH port for connecting to static nodes. Defaults to 22.
    nodes:
      - endpoint: "192.168.10.1"
        secretRef:
          name: static-node-key
          namespace: <your-namespace>
```
  
- Gracefully handling missing Cloudflare Load Balancing [#2029](https://github.com/berops/claudie/pull/2029)
  
- Dynamic nodes within a Kubernetes cluster will now be healthchecked by Claudie and if they're unhealthy for more than 12 mins Claudie will trigger an auto-repair mechanism
  in which the node is replaced by first deleting it and subsequently joining a new node into the cluster. [#2038](https://github.com/berops/claudie/pull/2038)

