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

# v0.1.2

## Features

- Update to Go `v1.20` [#559](https://github.com/Berops/claudie/pull/559)
- The VPN now respects netmask from defined CIDR [#571](https://github.com/Berops/claudie/pull/571)
- Connections attempt are more readable [#570](https://github.com/Berops/claudie/pull/570)

## Bugfixes

- Wireguard IP now persists across reboots [#557](https://github.com/Berops/claudie/pull/557)
- Deletion of the infrastructure before any outputs were created does not end with error [#569](https://github.com/Berops/claudie/pull/569)
- Replace the `azurerm_virtual_machine` to the `azurerm_linux_virtual_machine` [#573](https://github.com/Berops/claudie/pull/573)

## Known issues

- Longhorn replicas are  not properly managed, which might cause issues when deleting nodes [#564](https://github.com/Berops/claudie/issues/564)
- Naming scheme in input manifest is not uniform [#563](https://github.com/Berops/claudie/issues/563)

# v0.1.1

## Features

- Support DNS zone for Cloudflare, AWS, Azure, HetznerDNS, OCI [#530](https://github.com/Berops/claudie/pull/530)
- Add default node labels [#543](https://github.com/Berops/claudie/pull/543)

## Bugfixes

- Logs in all services have been modified to not output sensitive information. [#535](https://github.com/Berops/claudie/pull/535)
- Correctly update desiredState after workflow for a given manifest completes. [#536](https://github.com/Berops/claudie/pull/536)

## Known issues

- Wireguard interface `wg0` is missing ip address after reboot. Will be fixed in next release [#557](https://github.com/Berops/claudie/pull/557)

# v0.1.0

## Features

- Multi-cloud kubernetes cluster management
- Multi-cloud loadbalancer management
- Fast scale-up/scale-down of defined infrastructure
- Persistent storage via Longhorn
- Support for AWS, Azure, GCP, OCI and Hetzner
- GCP DNS zone support
- Claudie deployment on `amd64` and `arm64` clusters

## Bugfixes

- As this is first release there are no bugfixes

## Known issues

- `iptables` reset after reboot and block all traffic on `OCI` node [#466](https://github.com/Berops/claudie/issues/466)
- Occasional connection issues between Claudie created clusters and Claudie on Hetzner and GCP [#276](https://github.com/Berops/claudie/issues/276)
- Unable to easily recover after error [#528](https://github.com/Berops/claudie/issues/528)
