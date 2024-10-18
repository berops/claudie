# Claudie `v0.9`

!!! warning "Due to changes to the core of how Claudie works with terraform files and representation of the data in persistent storage the `v0.9.x` version will not be backwards compatible with clusters build using previous Claudie versions."

## Most notable changes (TL;DR)

- Support for pluggable external terraform files was added, breaking the dependency of updated terraform files on a new Claudie version. The ability to arbitrarily change the templates used by Claudie was made available to the user. As a result, Claudie has implemented a rolling update of the infrastructure in case a change in the terraform templates is detected, by gradually updating the build cluster one nodepool at a time. 
- Merged the Scheduler and Context-box service into a single called Manager.
- Each Nodepool now has its own SSH keys instead of sharing a single SSH key per kubernetes cluster.

### Experimental
- We have added support for an HTTP proxy to be used when building Kubernetes clusters. This was mainly motivated by the issues we encountered while building multi-provider clusters, where some IP addresses assigned to some of the VMs were being misused and blacklisted/blocked in various registries. By using the HTTP proxy, it is possible to work around this and get the cluster built successfully.

Currently the HTTP proxy is experimental, it is made available by modifying the `HTTP_PROXY_MODE` in the Claudie config map in the `Claudie` namespace. The possible values are `(on|off|default)`. Default means that if a kubernetes cluster uses Hetzner nodepools, it will automatically switch to using the proxy, as we have encountered the most bad IP issues with Hetzner. By default the proxy is turned off.

It should be noted that the proxy is still in an experimental phase, where the API for interacting with the proxy may change in the future. Therefore, clusters using this feature in this release run the risk of being backwards incompatible with future `0.9.x` releases, which will further stabilise the proxy API.

## Deployment

To deploy Claudie `v0.9.X`, please:

1. Download Claudie.yaml from [release page](https://github.com/berops/Claudie/releases)

2. Verify the checksum with `sha256` (optional)

   We provide checksums in `claudie_checksum.txt` you can verify the downloaded yaml files againts the provided checksums.

3. Install Claudie using `kubectl`

> We strongly recommend changing the default credentials for MongoDB, MinIO and DynamoDB before you deploy it.

```
kubectl apply -f https://github.com/berops/Claudie/releases/latest/download/Claudie.yaml
```

To further harden Claudie, you may want to deploy our pre-defined network policies:
   ```bash
   # for clusters using cilium as their CNI
   kubectl apply -f https://github.com/berops/Claudie/releases/latest/download/network-policy-cilium.yaml
   ```
   ```bash
   # other
   kubectl apply -f https://github.com/berops/Claudie/releases/latest/download/network-policy.yaml
   ```


## v0.9.0


## What's changed
- Support added for Ubuntu 24.04 in Azure and Hetzner [#1401](https://github.com/berops/Claudie/pull/1401)

- Each nodepool now has its own SSH keys, a change from the previous state where all nodepools shared the same SSH keys.. [#1442](https://github.com/berops/Claudie/pull/1442)

- Added support for pluggable external terraform files, breaking the dependency of updated terraform files on a new Claudie version. [#1460](https://github.com/berops/Claudie/pull/1460)

- With the support of external terraform templates, the ability to arbitrarily change the templates used by Claudie was made available to the user. As a result, Claudie has implemented a rolling update of the infrastructure in case a change in the terraform templates is detected, by gradually updating the build cluster one nodepool at a time. [#1525](https://github.com/berops/Claudie/pull/1525)

- The Scheduler and Context-Box microservices were merged into a single service called Manager. This was done because these two services were tightly coupled, and parts of the context box service were causing state correctness issues within Claudie and needed to be fixed. [#1498](https://github.com/berops/Claudie/pull/1498)

- Latest supported kubernetes version is now  v1.30.x [#1498](https://github.com/berops/Claudie/pull/1501)

- Logs in all microservices have been changed to always log what is being executed, rather than only when the LOG_LEVEL is set to debug. [#1507](https://github.com/berops/Claudie/pull/1507)

- Longhron version was bumped from 1.6.0 to 1.7.0 [#1511](https://github.com/berops/Claudie/pull/1511)

- When building a Kubernetes cluster without a load balancer for the API server, the generated kubeconfig will now work for all control plane nodes defined in the input manifest, instead of just one. [#1546](https://github.com/berops/Claudie/pull/1546)


### Experimental
- Support for a HTTP proxy was added. The HTTP Proxy can be turned on by setting the `HTTP_PROXY_MODE` environment variable in the Claudie config map to `on`  [#1440](https://github.com/berops/Claudie/pull/1440)

## Bug fixes
- In the case when the infrastructure fails to be build or is only partially build
  the deletion process was stuck on acquiring a lock which was never created, this issue is no longer present [#1463](https://github.com/berops/Claudie/pull/1463)
- The init process was added to the Ansible microservice because previously spawned Ansible playbooks left behind zombie processes that consumed resources. The init process takes care of cleaning up these processes. [#1527](https://github.com/berops/Claudie/pull/1527)
- Fixed an edge case where part of the load balancer infrastructure was incorrectly destroyed when a failure occurred in the middle of the workflow. [#1533](https://github.com/berops/Claudie/pull/1533)
- The whitespace when generating keys will no longer be trimmed [#1539](https://github.com/berops/Claudie/pull/1539)
- GenesisCloud autoscaling will now correctly work [#1543](https://github.com/berops/Claudie/pull/1543)
