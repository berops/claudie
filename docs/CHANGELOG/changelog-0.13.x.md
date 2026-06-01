# Claudie `v0.13`

!!! warning "Upgrade to this release from previous `v0.12` version requires manual intervention (due to the MongoDB version upgrade and changes to the CNI)."

## Readme Before Deploying 

### Kubernetes version changes

The support for kubernetes `v1.32.x` has been dropped. The currently supported versions are `v1.33`,`v1.34`,`v1.35`.
Make sure to update your kubernetes version to at least `v1.33`, before updating to this new release.

### MongoDB

This version of Claudie updates MongoDB. 

It is required that before deploying this new version that the number of
replicas of the current deployment for MongoDB is scaled down to 0.

```bash
kubectl scale deploy/mongodb -n claudie --replicas=0
```


After deploying:

- Check if the version running in the primary replica is 7.0 via

```bash
kubectl exec -it <primary-mongo-pod> -n claudie -- mongosh \
  -u <username> -p <password> --authenticationDatabase admin \
  --eval "db.adminCommand({ buildInfo: 1 }).version"
```

- Set the feature set to version 7.0

```bash
kubectl exec -it <primary-mongo-pod> -n claudie -- mongosh \
  -u <username> -p <password> --authenticationDatabase admin \
  --eval "db.adminCommand( { setFeatureCompatibilityVersion: '7.0', confirm: true } )"
```

This command must perform writes to an internal system collection. If for any reason the command does not complete successfully, you can safely retry the command as the operation is idempotent.

- Verify the command successfully updated the feature set.

```bash
kubectl exec -it <primary-mongo-pod> -n claudie -- mongosh \
  -u <username> -p <password> --authenticationDatabase admin \
  --eval "db.adminCommand({ getParameter: 1, featureCompatibilityVersion: 1 })"
```

### CNI changes

After deploying this version of Claudie, the next scheduled task that passes through
the `kube-eleven` stage will update the CNI, during which `kube-proxy` will be removed
and replaced by cilium-cni running in `eBPF` mode.

**This update may introduce a short downtime of ~5-10mins, while the `kube-proxy` is being replaced**

Note that claudie has a periodic refresh of the infrastructure every ~30min, thus if until then no task
is scheduled that passes through the `kube-eleven` stage, this periodic refresh will
take care of it.

## Deployment

To deploy Claudie `v0.13.x`, please:

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
- Updated MongoDB to version 7 [#2075](https://github.com/berops/claudie/pull/2075)

    - Before deploying scale down MongoDB to 0 replicas.
```bash
kubectl scale deploy/mongodb -n claudie --replicas=0
```

    - After deploying, check if the version running in the primary replica is 7.0
```bash
kubectl exec -it <primary-mongo-pod> -n claudie -- mongosh \
  -u <username> -p <password> --authenticationDatabase admin \
  --eval "db.adminCommand({ buildInfo: 1 }).version"
```

    - Set the feature set to version 7.0
```bash
kubectl exec -it <primary-mongo-pod> -n claudie -- mongosh \
  -u <username> -p <password> --authenticationDatabase admin \
  --eval "db.adminCommand( { setFeatureCompatibilityVersion: '7.0', confirm: true } )"
```
This command must perform writes to an internal system collection. If for any reason the command does not complete successfully, you can safely retry the command as the operation is idempotent.

    - Verify the command successfully updated the feature set.
```bash
kubectl exec -it <primary-mongo-pod> -n claudie -- mongosh \
  -u <username> -p <password> --authenticationDatabase admin \
  --eval "db.adminCommand({ getParameter: 1, featureCompatibilityVersion: 1 })"
```

- The **support for kubernetes v1.32.0 is dropped**. The currently supported version are: v1.33, v1.34, v1.35. Make sure to update to at least `v1.33.x` before deploying.[#2079](https://github.com/berops/claudie/pull/2079)

- Added support for a new cloud provider Verda Cloud [#2088](https://github.com/berops/claudie/pull/2088)

- Added support for OVH as native compute and DNS provider [#2117](https://github.com/berops/claudie/pull/2117)

- Cilium will now be deployed in `eBPF` mode [#2091](https://github.com/berops/claudie/pull/2091) [#2109](https://github.com/berops/claudie/pull/2109)

- General maintanance update of dependnecies. [#2080](https://github.com/berops/claudie/pull/2080), [#2107](https://github.com/berops/claudie/pull/2107)

- Fixed some of the workflow ordering in scheduled tasks [#2115](https://github.com/berops/claudie/pull/2115)
