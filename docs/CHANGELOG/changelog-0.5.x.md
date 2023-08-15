# Claudie `v0.5`

!!! warning "Due to a breaking change in swapping the CNI used in the Kubernetes cluster, the `v0.5.x` will not be backwards compatible with `v0.4.x`"

## Deployment

To deploy Claudie `v0.5.X`, please:

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

## v0.5.0

### Features

- Use cilium as the cluster CNI [#984](https://github.com/berops/claudie/pull/984)
- Update to the latest longhorn version v1.5.1 [#984](https://github.com/berops/claudie/pull/984)

### Known issues

- No known issues since the last release

## v0.5.1

### Bug fixes

- Fix issue when node deletion from the cluster wouldn't be idempotent [#1008](https://github.com/berops/claudie/pull/1008)
