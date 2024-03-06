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

## Bugfixes
* Static Loadbalancer metadata secret https://github.com/berops/claudie/pull/1249
* Update healthcheck endpoints https://github.com/berops/claudie/pull/1245
