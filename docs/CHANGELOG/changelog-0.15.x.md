# Claudie `v0.15`

!!! warning "Upgrade to this release from previous `v0.14.x` version requires manual intervention (due to the MongoDB version upgrade)."

## Readme Before Deploying 

### Autoscaler changes

The autoscaler will no longer by deployed by Claudie, and will now be an opt-in feature behind a paywall.

### Make sure no tasks are scheduled

Before deploying this Claudie release, make sure no tasks are actively being worked on. You can verify this via

```bash
kubectl get inputmanifests
```

All of the input manifests should be in state `WATCHING_FOR_CHANGES`.

You can also scale down the number of replicas for the manager service to 0 to be sure no tasks are scheduled.

```bash
kubectl scale deploy/manager -n claudie --replicas=0
```

### MongoDB

This version of Claudie updates MongoDB. 

It is required that before deploying this new version that the number of
replicas of the current deployment for MongoDB is scaled down to 0.

```bash
kubectl scale deploy/mongodb -n claudie --replicas=0
```


After deploying:

- Check if the version running in the primary replica is 8.3 via

```bash
kubectl exec -it <primary-mongo-pod> -n claudie -- mongosh \
  -u <username> -p <password> --authenticationDatabase admin \
  --eval "db.adminCommand({ buildInfo: 1 }).version"
```

- Set the feature set to version 8.3

```bash
kubectl exec -it <primary-mongo-pod> -n claudie -- mongosh \
  -u <username> -p <password> --authenticationDatabase admin \
  --eval "db.adminCommand( { setFeatureCompatibilityVersion: '8.3', confirm: true } )"
```

This command must perform writes to an internal system collection. If for any reason the command does not complete successfully, you can safely retry the command as the operation is idempotent.

- Verify the command successfully updated the feature set.

```bash
kubectl exec -it <primary-mongo-pod> -n claudie -- mongosh \
  -u <username> -p <password> --authenticationDatabase admin \
  --eval "db.adminCommand({ getParameter: 1, featureCompatibilityVersion: 1 })"
```

## Deployment

To deploy Claudie `v0.15.x`, please:

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

- Updated mongo to version 8.3 [#2141](https://github.com/berops/claudie/pull/2141)

- Cluster-autoscaler has been removed and is now opt-in behind a paywall [#2146](https://github.com/berops/claudie/pull/2146)

- Added support spot instances on AWS, Azure, OCI, Verda, GCp [#2151](https://github.com/berops/claudie/pull/2151), [#2149](https://github.com/berops/claudie/pull/2149), [#2145](https://github.com/berops/claudie/pull/2145)

- Faster handling of nodes with unknown status [#2150](https://github.com/berops/claudie/pull/2150)

- Fixed nil panic on DNS creation failure [#2152](https://github.com/berops/claudie/pull/2152)

- Updated template parsing for alternative names [#2153](https://github.com/berops/claudie/pull/2153)
