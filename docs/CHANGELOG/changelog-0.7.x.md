# Claudie `v0.7`

!!! warning "Due to using the latest version of longhorn the `v0.7.x` will not be backwards compatible with `v0.6.x`"

## Deployment

To deploy Claudie `v0.7.X`, please:

1. Download claudie.yaml from [release page](https://github.com/berops/claudie/releases)

2. Verify the checksum with `sha256` (optional)
   
    We provide checksums in `claudie_checksum.txt` you can verify the downloaded yaml files againts the provided checksums.

3. Install claudie using `kubectl`

 > We strongly recommend changing the default credentials for MongoDB, MinIO and DynamoDB before you deploy it.

```
kubectl apply -f https://github.com/berops/claudie/releases/latest/download/claudie.yaml
```
   
   To further harden claudie, you may want to deploy our pre-defined network policies:
   ```bash
   # for clusters using cilium as their CNI
   kubectl apply -f https://github.com/berops/claudie/releases/latest/download/network-policy-cilium.yaml
   ```
   ```bash
   # other
   kubectl apply -f https://github.com/berops/claudie/releases/latest/download/network-policy.yaml
   ```



## v0.7.0

*Upgrade procedure:*
Before upgrading Claudie, upgrade Longhorn to 1.6.x as per [this guide](https://longhorn.io/docs/1.6.0/deploy/upgrade/longhorn-manager/#upgrade-with-kubectl-1). In most cases this will boil down to running the following command:
`kubectl apply -f https://raw.githubusercontent.com/longhorn/longhorn/v1.6.0/deploy/longhorn.yaml`.


### Features
- Add possibility to use external s3/dynamo/mongo instances [#1191](https://github.com/berops/claudie/pull/1191)
- Add Genesis Cloud support [#1210](https://github.com/berops/claudie/pull/1210)
- Add annotations support for nodepools in Input Manifest [#1238](https://github.com/berops/claudie/pull/1238)
- Update Longhorn to latest version [#1213](https://github.com/berops/claudie/pull/1213)
### Bugfixes
- Fix removing state lock from dynamodb [#1211](https://github.com/berops/claudie/pull/1211)
- Fix operatur status message [#1215](https://github.com/berops/claudie/pull/1215)
- Fix custom storage classes [#1219](https://github.com/berops/claudie/pull/1219)

## v0.7.1

Migrate from the legacy package repositories `apt.kubernetes.io, yum.kubernetes.io` to the Kubernetes community-hosted repositories `pkgs.k8s.io`.
A detailed how to can be found in [https://kubernetes.io/blog/2023/08/31/legacy-package-repository-deprecation/](https://kubernetes.io/blog/2023/08/31/legacy-package-repository-deprecation/)

Kubernetes version 1.24 is no longer supported.
1.25.x 1.26.x 1.27.x are the currently supported versions.

## Bugfixes
* Static Loadbalancer metadata secret [#1249](https://github.com/berops/claudie/pull/1249)
* Update healthcheck endpoints [#1245](https://github.com/berops/claudie/pull/1245)

## v0.7.2
### Features
* Target Nodepools [#1241](https://github.com/berops/claudie/pull/1241)

## v0.7.3
### Bugfixes
- Fix endless queueing of manifests with static nodepools, [#1282](https://github.com/berops/claudie/pull/1282)


## v0.7.4
### Bugfixes
- Loadbalancer Endpoint missing in current state when workflow fails [#1284](https://github.com/berops/claudie/pull/1284)
- Prevent autoscaling request when manifest is in error [#1288](https://github.com/berops/claudie/pull/1288)
- Update healthchecks for builder that resulting in frequent restarts [#1293](https://github.com/berops/claudie/pull/1293)

## v0.7.5
### Features
- increase worker_connections per worker process for load balancers [#1328](https://github.com/berops/claudie/pull/1328)

### Bugifxes
- Fix connection issues across services [#1331](https://github.com/berops/claudie/pull/1331)
