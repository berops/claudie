# Claudie `v0.3`

!!! warning "Due to a breaking change in the input manifest schema, the `v0.3.x` will not be backwards compatible with `v0.2.x`"

## Deployment

To deploy the Claudie `v0.3.X`, please:

1. Download the archive and checksums from the [release page](https://github.com/berops/claudie/releases)

2. Verify the archive with the `sha256` (optional)

    ```sh
    sha256sum -c --ignore-missing checksums.txt
    ```

    If valid, output is, depending on the archive downloaded

    ```sh
    claudie.tar.gz: OK
    ```

    or

    ```sh
    claudie.zip: OK
    ```

    or both.

3. Lastly, unpack the archive and deploy using `kubectl`

    > We strongly recommend changing the default credentials for MongoDB, MinIO and DynamoDB before you deploy it. To do this, change contents of the files in `mongo/secrets`, `minio/secrets` and `dynamo/secrets` respectively.

    ```sh
    kubectl apply -k .
    ```

## v0.3.0

### Features

- Use separate storage disk for longhorn [#689](https://github.com/berops/claudie/pull/698)
- Apply proper kubernetes labels to Claudie resources [#714](https://github.com/berops/claudie/pull/714)
- Implement clean architecture for the Frontend [#701](https://github.com/berops/claudie/pull/701)

### Bugfixes

- Fix logging issues in Frontend [#713](https://github.com/berops/claudie/pull/713)

### Known issues

- Infrastructure might not get deleted if workflow encounters and error [#712](https://github.com/berops/claudie/issues/712)
- Certain cluster manipulation can result in workflow failing to build the clusters [#606](https://github.com/berops/claudie/issues/606)

## v0.3.1

### Features

- Rework logs in all microservices to enable easier filtering [#742](https://github.com/berops/claudie/pull/742)
- Improve longhorn volume replication management [#782](https://github.com/berops/claudie/pull/782)
- Various improvements in cluster manipulation [#728](https://github.com/berops/claudie/pull/728)
- Removal of `k8s-sidecar` from Frontend [#792](https://github.com/berops/claudie/pull/792)

### Bugfixes

- Fixed bug when infrastructure was not deleted if workflow encountered an error [#773](https://github.com/berops/claudie/pull/773)
- Fixed error when deletion of nodes from cluster failed [#728](https://github.com/berops/claudie/pull/728)
- Fixed bug when frontend triggered deletion of incorrect manifest [#744](https://github.com/berops/claudie/pull/744)

### Known issues

- Subnet CIDR is not carried over from temporary state in Builder [#790](https://github.com/berops/claudie/issues/790)
- Longhorn occasionally does not detach volume from node which was deleted [#784](https://github.com/berops/claudie/issues/784)

## v0.3.2

### Features

- Label Claudie output secrets [#837](https://github.com/berops/claudie/pull/837)
- Use cluster name in the output kubeconfig context [#830](https://github.com/berops/claudie/pull/830)
- Make DynamoDB job idempotent [#817](https://github.com/berops/claudie/pull/817)
- Implement secret validation webhook [#821](https://github.com/berops/claudie/pull/821)
- Improve Cluster Autoscaler deployment [#805](https://github.com/berops/claudie/pull/805)

### Bugfixes

- Fixed bug when subnet CIDR is not carried over from temporary state in Builder [#812](https://github.com/berops/claudie/pull/812)

### Known issues

No known issues since the last release.
