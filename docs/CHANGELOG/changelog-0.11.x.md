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
