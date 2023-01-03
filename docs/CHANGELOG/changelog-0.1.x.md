# v0.1.0

First official release of Claudie

# Features

- Multi-cloud kubernetes cluster management
- Multi-cloud loadbalancer management
- Fast scale-up/scale-down of defined infrastructure
- Persistent storage via Longhorn
- Support for AWS, Azure, GCP, OCI and Hetzner
- Support for Cloud DNS on GCP only (for now)

# Bugfixes
- As this is first release there are no bugfixes

# Known issues
- `Terraformer` sometimes timeouts when provisioning `Azure` VMs #386
- `iptables` reset after reboot and block all traffic on `OCI` node #466
- Numerous issues when deploying on `arm64` based machines 