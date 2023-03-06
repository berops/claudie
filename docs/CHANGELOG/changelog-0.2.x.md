# Claudie `v0.2`

:warning: Due to a breaking change in the input manifest schema, the `v0.2.x` will not be backwards compatible with `v0.1.x`. :warning:

# Deployment

To deploy the Claudie `v0.2.X`, please:

1. Download the archive and checksums from the [release page](https://github.com/Berops/claudie/releases)

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

# v0.2.0

## Features

- Unify the naming schema in the input manifest [#601](https://github.com/berops/claudie/pull/601)
- Deploy MinIO in multi-replica fashion [#589](https://github.com/berops/claudie/pull/589)

## Bugfixes

No bugfixes since the last release.

## Known issues

- Workflow fails to build when a user makes multiple changes of the input manifest, regarding the API endpoint [#606](https://github.com/berops/claudie/issues/606)
- Longhorn pod longhorn-admission-webhook stuck in Init state [#598](https://github.com/berops/claudie/issues/598)
- Deletion of config fails if builder crashes after deleting nodes [#588](https://github.com/berops/claudie/issues/588)
