# Claudie `v0.4`

!!! warning "Due to a breaking change in the input manifest schema, the `v0.4.x` will not be backwards compatible with `v0.3.x`"

## Deployment

To deploy the Claudie `v0.4.X`, please:

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

## v0.4.0

### Features

- Input manifest definition now uses CRD instead of secret [#872](https://github.com/berops/claudie/pull/872)
- Various improvements in the overall documentation [#864](https://github.com/berops/claudie/pull/864) [#871](https://github.com/berops/claudie/pull/871) [#884](https://github.com/berops/claudie/pull/884) [#888](https://github.com/berops/claudie/pull/888)  [#891](https://github.com/berops/claudie/pull/891) [#893](https://github.com/berops/claudie/pull/893)

### Bugfixes

- Errors from the Scheduler are correctly saved under the clusters state [#868](https://github.com/berops/claudie/pull/868)
- Failure in the Terraformer will correctly saves the created state [#875](https://github.com/berops/claudie/pull/875)
- The clusters which previously resulted in error no longer block the workflow on input manifest reapply [#883](https://github.com/berops/claudie/pull/883)

### Known issues

- Single node pool definition cannot be used as control plane and as compute plane in the same cluster. [#865](https://github.com/berops/claudie/issues/865)
- Input manifest status is not tracked during autoscaling [#886](https://github.com/berops/claudie/issues/886)
