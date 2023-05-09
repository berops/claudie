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