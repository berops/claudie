# Claudie `v0.12`

!!! warning "Upgrade to this release from previous `v0.11` version requires manual intervention (due to the MongoDB version upgrade)."

## Deployment

To deploy Claudie `v0.12.x`, please:

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


## v0.12.0

## What's Changed

- Changing credentials for providers will now be correctly propagated within the reconciliation loop [#2056](https://github.com/berops/claudie/pull/2056)

- Updated MongoDB to version 6.0 [#2053](https://github.com/berops/claudie/pull/2053)

    - After deploying, verify Mongo version is `6.0`
```
kubectl exec -it <primary-mongo-pod> -n claudie -- mongosh \
  -u <username> -p <password> --authenticationDatabase admin \
  --eval "db.adminCommand({ buildInfo: 1 }).version"
```
  
    - Manually set the feature set to version `6.0`
```
kubectl exec -it <primary-mongo-pod> -n claudie -- mongosh \
  -u <username> -p <password> --authenticationDatabase admin \
  --eval "db.adminCommand({ setFeatureCompatibilityVersion: '6.0' })"
```
      This command must perform writes to an internal system collection. If for any reason the command does not complete successfully, you can safely retry the command as the operation is idempotent.

    - Verify the update was processed. The following command should return `6.0` for the feature set.
```
kubectl exec -it <primary-mongo-pod> -n claudie -- mongosh \
  -u <username> -p <password> --authenticationDatabase admin \
  --eval "db.adminCommand({ getParameter: 1, featureCompatibilityVersion: 1 })"
```



## Bug fixes
- Fixed deletion of zero sized nodepools that would result in an endless reconciliation loop [#2049](https://github.com/berops/claudie/pull/2049)
