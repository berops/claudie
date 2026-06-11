# Claudie `v0.14`

!!! warning "Upgrade to this release from previous `v0.13` version requires manual intervention (due to the MongoDB version upgrade)."

## Readme Before Deploying 

### Make sure no tasks are scheduled

Before deploying this Claudie release, make sure no tasks are actively being worked on. You can verify this via

```bash
kubectl get inputmanifests
```

All of the input manifests should be in state `WATCHING_FOR_CHANGES`.

You can also scale down the numbe of replicas for the manager service to 0 to be sure no tasks are scheduled.

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

- Check if the version running in the primary replica is 8.0 via

```bash
kubectl exec -it <primary-mongo-pod> -n claudie -- mongosh \
  -u <username> -p <password> --authenticationDatabase admin \
  --eval "db.adminCommand({ buildInfo: 1 }).version"
```

- Set the feature set to version 8.0

```bash
kubectl exec -it <primary-mongo-pod> -n claudie -- mongosh \
  -u <username> -p <password> --authenticationDatabase admin \
  --eval "db.adminCommand( { setFeatureCompatibilityVersion: '8.0', confirm: true } )"
```

This command must perform writes to an internal system collection. If for any reason the command does not complete successfully, you can safely retry the command as the operation is idempotent.

- Verify the command successfully updated the feature set.

```bash
kubectl exec -it <primary-mongo-pod> -n claudie -- mongosh \
  -u <username> -p <password> --authenticationDatabase admin \
  --eval "db.adminCommand({ getParameter: 1, featureCompatibilityVersion: 1 })"
```

## Deployment

To deploy Claudie `v0.14.x`, please:

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

- Updated MongoDB to version 8 [#2125](https://github.com/berops/claudie/pull/2125)

    - Before deploying scale down MongoDB to 0 replicas.
```bash
kubectl scale deploy/mongodb -n claudie --replicas=0
```

    - After deploying, check if the version running in the primary replica is 8.0
```bash
kubectl exec -it <primary-mongo-pod> -n claudie -- mongosh \
  -u <username> -p <password> --authenticationDatabase admin \
  --eval "db.adminCommand({ buildInfo: 1 }).version"
```

    - Set the feature set to version 8.0
```bash
kubectl exec -it <primary-mongo-pod> -n claudie -- mongosh \
  -u <username> -p <password> --authenticationDatabase admin \
  --eval "db.adminCommand( { setFeatureCompatibilityVersion: '8.0', confirm: true } )"
```
This command must perform writes to an internal system collection. If for any reason the command does not complete successfully, you can safely retry the command as the operation is idempotent.

    - Verify the command successfully updated the feature set.
```bash
kubectl exec -it <primary-mongo-pod> -n claudie -- mongosh \
  -u <username> -p <password> --authenticationDatabase admin \
  --eval "db.adminCommand({ getParameter: 1, featureCompatibilityVersion: 1 })"
```

- General dependencies maintenance [`#2126`](https://github.com/berops/claudie/pull/2126), [`#2132`](https://github.com/berops/claudie/pull/2132), [`#2136`](https://github.com/berops/claudie/pull/2136)

- Adjusted Claudie internal retries for autoscaled nodepools to fail faster [#2129](https://github.com/berops/claudie/pull/2129)

- Support for shared public IP nodes has been added [#2130](https://github.com/berops/claudie/pull/2130)
