# Claudie `v0.1`

The first official release of Claudie

# Deployment

To deploy the Claudie `v0.1.X`, please:

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

# v0.1.0

## Features

- Multi-cloud kubernetes cluster management
- Multi-cloud loadbalancer management
- Fast scale-up/scale-down of defined infrastructure
- Persistent storage via Longhorn
- Support for AWS, Azure, GCP, OCI and Hetzner
- Support for Cloud DNS on GCP only
- Claudie deployment on `amd64` and `arm64` clusters

## Bugfixes

- As this is first release there are no bugfixes

## Known issues

- `iptables` reset after reboot and block all traffic on `OCI` node [#466](https://github.com/Berops/claudie/issues/466)
- Occasional connection issues between Claudie created clusters and Claudie on Hetzner and GCP [#276](https://github.com/Berops/claudie/issues/276)
- Unable to easily recover after error [#528](https://github.com/Berops/claudie/issues/528)
