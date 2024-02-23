# Claudie `v0.7`

!!! warning "Due to using the latest version of longhorn the `v0.7.x` will not be backwards compatible with `v0.6.x`"

## Deployment

To deploy Claudie `v0.7.X`, please:

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



## v0.7.0

If you already have clusters deployed, you need to manually upgrade longhorn to version 1.6.0 ([see](https://longhorn.io/docs/1.6.0/deploy/upgrade/longhorn-manager/#upgrade-with-kubectl-1)), otherwise claudie will fail when a workflow is started for a cluster build using older 0.6.x versions.

### Features
- Add possibility to use external s3/dynamo/mongo instances [#1191](https://github.com/berops/claudie/pull/1191)
- Add Genesis Cloud support [#1210](https://github.com/berops/claudie/pull/1210)
- Add annotations support for nodepools in Input Manifest [#1238](https://github.com/berops/claudie/pull/1238)
- Update Longhorn to latest version [#1213](https://github.com/berops/claudie/pull/1213)
### Bugfixes
- Fix removing state lock from dynamodb [#1211](https://github.com/berops/claudie/pull/1211)
- Fix operatur status message [#1215](https://github.com/berops/claudie/pull/1215)
- Fix custom storage classes [#1219](https://github.com/berops/claudie/pull/1219)
