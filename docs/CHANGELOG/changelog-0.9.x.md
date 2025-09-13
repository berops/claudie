# Claudie `v0.9`

!!! warning "Due to changes to the core of how Claudie works with terraform files and representation of the data in persistent storage the `v0.9.x` version will not be backwards compatible with clusters build using previous Claudie versions."

## Most notable changes (TL;DR)

- Support for pluggable external terraform files was added, breaking the dependency of updated terraform files on a new Claudie version. The ability to arbitrarily change the templates used by Claudie was made available to the user. As a result, Claudie has implemented a rolling update of the infrastructure in case a change in the terraform templates is detected, by gradually updating the build cluster one nodepool at a time. 
- Merged the Scheduler and Context-box service into a single called Manager.
- Each Nodepool now has its own SSH keys instead of sharing a single SSH key per kubernetes cluster.

### Experimental
- We have added support for an HTTP proxy to be used when building Kubernetes clusters. This was mainly motivated by the issues we encountered while building multi-provider clusters, where some IP addresses assigned to some of the VMs were being misused and blacklisted/blocked in various registries. By using the HTTP proxy, it is possible to work around this and get the cluster built successfully.

Currently the HTTP proxy is experimental, it is made available by modifying the `HTTP_PROXY_MODE` in the Claudie config map in the `claudie` namespace. The possible values are `(on|off|default)`. Default means that if a kubernetes cluster uses Hetzner nodepools, it will automatically switch to using the proxy, as we have encountered the most bad IP issues with Hetzner. By default the proxy is turned off.

It should be noted that the proxy is still in an experimental phase, where the API for interacting with the proxy may change in the future. Therefore, clusters using this feature in this release run the risk of being backwards incompatible with future `0.9.x` releases, which will further stabilise the proxy API.

## Deployment

To deploy Claudie `v0.9.X`, please:

1. Download claudie.yaml from [release page](https://github.com/berops/claudie/releases)

2. Verify the checksum with `sha256` (optional)

   We provide checksums in `claudie_checksum.txt` you can verify the downloaded yaml files againts the provided checksums.

3. Install Claudie using `kubectl`

> We strongly recommend changing the default credentials for MongoDB, MinIO and DynamoDB before you deploy it.

```
kubectl apply -f https://github.com/berops/claudie/releases/latest/download/claudie.yaml
```

To further harden Claudie, you may want to deploy our pre-defined network policies:
   ```bash
   # for clusters using cilium as their CNI
   kubectl apply -f https://github.com/berops/claudie/releases/latest/download/network-policy-cilium.yaml
   ```
   ```bash
   # other
   kubectl apply -f https://github.com/berops/claudie/releases/latest/download/network-policy.yaml
   ```


## v0.9.0


## What's changed
- Support added for Ubuntu 24.04 in Azure and Hetzner [#1401](https://github.com/berops/claudie/pull/1401)

- Each nodepool now has its own SSH keys, a change from the previous state where all nodepools shared the same SSH keys.. [#1442](https://github.com/berops/claudie/pull/1442)

- Added support for pluggable external terraform files, breaking the dependency of updated terraform files on a new Claudie version. [#1460](https://github.com/berops/claudie/pull/1460)

- With the support of external terraform templates, the ability to arbitrarily change the templates used by Claudie was made available to the user. As a result, Claudie has implemented a rolling update of the infrastructure in case a change in the terraform templates is detected, by gradually updating the build cluster one nodepool at a time. [#1525](https://github.com/berops/claudie/pull/1525)

- The Scheduler and Context-Box microservices were merged into a single service called Manager. This was done because these two services were tightly coupled, and parts of the context box service were causing state correctness issues within Claudie and needed to be fixed. [#1498](https://github.com/berops/claudie/pull/1498)

- Latest supported kubernetes version is now  v1.30.x [#1498](https://github.com/berops/claudie/pull/1501)

- Logs in all microservices have been changed to always log what is being executed, rather than only when the LOG_LEVEL is set to debug. [#1507](https://github.com/berops/claudie/pull/1507)

- Longhron version was bumped from 1.6.0 to 1.7.0 [#1511](https://github.com/berops/claudie/pull/1511)

- When building a Kubernetes cluster without a load balancer for the API server, the generated kubeconfig will now work for all control plane nodes defined in the input manifest, instead of just one. [#1546](https://github.com/berops/claudie/pull/1546)


### Experimental
- Support for a HTTP proxy was added. The HTTP Proxy can be turned on by setting the `HTTP_PROXY_MODE` environment variable in the Claudie config map to `on`  [#1440](https://github.com/berops/claudie/pull/1440)

## Bug fixes
- In the case when the infrastructure fails to be build or is only partially build
  the deletion process was stuck on acquiring a lock which was never created, this issue is no longer present [#1463](https://github.com/berops/claudie/pull/1463)
- The init process was added to the Ansible microservice because previously spawned Ansible playbooks left behind zombie processes that consumed resources. The init process takes care of cleaning up these processes. [#1527](https://github.com/berops/claudie/pull/1527)
- Fixed an edge case where part of the load balancer infrastructure was incorrectly destroyed when a failure occurred in the middle of the workflow. [#1533](https://github.com/berops/claudie/pull/1533)
- The whitespace when generating keys will no longer be trimmed [#1539](https://github.com/berops/claudie/pull/1539)
- GenesisCloud autoscaling will now correctly work [#1543](https://github.com/berops/claudie/pull/1543)


## v0.9.1

## What's Changed
- Allow to overwrite the following default labels for static nodepools, which enables more customization for the static nodepools [#1550](https://github.com/berops/claudie/pull/1550)
    ```
        claudie.io/provider=static-provider 
        claudie.io/provider-instance=static-provider
        topology.kubernetes.io/region=static-region
        topology.kubernetes.io/zone=static-zone
    ```
- In the previous release proxy was introduced as an experimental feature. This release further stabilizes the proxy interface by introducing the following options to be set within the InputManifest [#1540](https://github.com/berops/claudie/pull/1540)
    ```
        kubernetes:
        clusters:
          - name: proxy-example
            version: "1.30.0"
            network: 192.168.2.0/24
            installationProxy:
                mode: "(on|off|default)"
    ```
  - On  - proxy will be used across all nodes in the cluster at all times.
  - Off - proxy will be turned of across the cluster.
  - Default - proxy will be turned on across the cluster for all nodes if the cluster contains at least one hetzner node.
  
    NOTE: if your cluster was build with the proxy turned on during the experimental phase, this change may or may not work, create backups before updating to the new version.


- When triggering a change of the the API endpoint of a cluster, an endless retry was added to the task executing the change as in the case of an error the cluster would endup malformed. This change will require user intervention to fix the underlying issue, if any occurs [#1577](https://github.com/berops/claudie/pull/1577)


- Basic reconciliation was added for autoscaled events in case of an error during the execution [#1582](https://github.com/berops/claudie/pull/1582)
    - If error occurs during the addition of the node, claudie will rollback by deleting the added node and any associated infrastructure
    - If errors occurs during the deletion of the node, claudie will retry the deletion multiple times
  
    For both of the cases it will retry the rollback or deletion of the node multiple times with an exponential backoff with up to an hour.

## Bug fixes
- Up until now, if there was any invalid input in the InputManifest or the infrastructure was able to be only partially created, the InputManifest would end up with an error where only manual deletion would help to remove the partially constructed infrastructure, This was fixed, so that if anything fails during the addition of new infrastructure into the cluster, claudie will rollback to the last working point, by removing the partially created infrastructure [#1566](https://github.com/berops/claudie/pull/1566)
 
- Longhorn related issues, especially during node deletion resulted in many InputManifest issues, In this release we fixed the issues by switching to a different drain policy for longhorn replicas deployed across the nodes on the cluster, namely `block-for-eviction-if-last-replica`[#1596](https://github.com/berops/claudie/pull/1596) which results in:
    - Protecting data by preventing the drain operation from completing until there is a healthy replica available for each volume available on another node.
    - Automatically evicts replicas, so the user does not need to do it manually.

 
## v0.9.2

## What's Changed
- Node local dns will be deployed on all newly build clusters [#1603](https://github.com/berops/claudie/pull/1603).
  For existing clusters that were build using older Claudie version, this change will deploy the `node-local-dns` into the cluster
  but it will not automatically work. Manual work needs to done, by first editing the `kubelet-config` ConfigMap in the `kube-system` namespace of the cluster
  to change the DNS address to the address of the `node-local-dns` and then on each node the following changes need to be done: [applying-kubelet-configuration-changes](https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/kubeadm-reconfigure/#reflecting-the-kubelet-changes).

## Bug fixes
- Improved validation errors when zero nodes are defined in a nodepool [#1605](https://github.com/berops/claudie/pull/1605)
- Claudie will now correctly recognize a change in the kubernetes version to perform an update [#1607](https://github.com/berops/claudie/pull/1607)
- Kubernetes secrets with provider credentials that contain leading or trailing whitespace will now be trimmed, avoiding issues with generated terraform templates [#1606](https://github.com/berops/claudie/pull/1606)
- Changing the API endpoint will now correctly work, after the recent kubeone version update [#1619](https://github.com/berops/claudie/pull/1619)

## v0.9.3

## Bug fixes
- Correctly turn HTTP proxy on/off [#1636](https://github.com/berops/claudie/pull/1636).
  HTTP proxy feature introduced in v0.9.1 was not working correctly mostly when switching between the on/off mode.

## v0.9.4

## Bug fixes
* Fix backwards compatibility with changes introduced in Claudie version 0.9.3 for clusters build using older versions 0.9.x [#1651](https://github.com/berops/claudie/pull/1651).
 If you built your cluster using the Claudie version 0.9.3, you can ignore this minor release.

## v0.9.5

## Bug fixes
- Correclty assign CIDR to loadbalancer nodepools [#1654](https://github.com/berops/claudie/pull/1654).
  This issue was prelevant mostly when working with loadbalancer from cloud providers that were not hetzner.

## v0.9.6

## Bug fixes
- Fixed issue where failing to build a load balancer would cause Claudie to hang if the DNS part failed [#1660](https://github.com/berops/claudie/pull/1660).
  Claudie will now recover from this scenario and it is possible for the user to specify the correct DNS settings in the InputManifest to fixed the reported issue.

## v0.9.7

## What's Changed
- Additional settings were added to roles for LoadBalancers. [#1685](https://github.com/berops/claudie/pull/1685).

  It is now possible to configure adding/removing proxy protocol and sticky sessions.

  `stickySessions` will always forward traffic to the same node based on the IP hash.
  
  `proxyProtocol` will turn on the proxy protocol. If used, the application to which the traffic is redirected must support this protocol.

  ```
    loadBalancers:
    roles:
      - name: example-role
        protocol: tcp
        port: 6443
        targetPort: 6443
        targetPools:
          - htz-kube-nodes
        # added
        settings:
          proxyProtocol: off (default will be on)
          stickySession: on. (default will be off)
  ```

## Bug fixes
- If any of the nodes become unreachable, Claudie will report the problem and will not work on any changes until the connectivity issue is resolved. [#1658](https://github.com/berops/claudie/pull/1658)
  
  For unreachable nodes within the kubernetes cluster, Claudie will give you the options of resolving the issue or removing the node from the InputManifest or via `kubectl`, Claudie will report the following issue
  ```
  fix the unreachable nodes by either:
   - fixing the connectivity issue
   - if the connectivity issue cannot be resolved, you can:
     - delete the whole nodepool from the kubernetes cluster in the InputManifest
     - delete the selected unreachable node/s manually from the cluster via 'kubectl'
       - if its a static node you will also need to remove it from the InputManifest
       - if its a dynamic node claudie will replace it.
       NOTE: if the unreachable node is the kube-apiserver, claudie will not be able to recover
             after the deletion.
  ```

  For unreachable nodes within the loadbalancer cluster, Claudie will give you the options of resolving the issue or removing the nodepool or load balancer from the InputManifest, Claudie will report the following issue
  ```
  fix the unreachable nodes by either:
   - fixing the connectivity issue
   - if the connectivity issue cannot be resolved, you can:
     - delete the whole nodepool from the loadbalancer cluster in the InputManifest
     - delete the whole loadbalancer cluster from the InputManifest
  ```

- It may be the case that the cluster-autoscaler image may not share the same version as the specified kubernetes version in the InputManifest. Claudie will now correctly recognize this and pick the latest available cluster-autoscaler image [#1680](https://github.com/berops/claudie/pull/1680)

- Claudie will now set the limits of max open file descriptors on each node to 65535 [#1679](https://github.com/berops/claudie/pull/1679)

## v0.9.8

## What's Changed
- Added support for alternative names for load balancers [#1693](https://github.com/berops/claudie/pull/1693)
  
  
  ```yaml
     dns:
       dnsZone: example.com
       provider: example
       hostname: main
       alternativeNames:
         - other
  ```

  Templates that Claudie uses by default, will be updated separately to make use of the alternative names.

## Bug fixes
- If the current state was not built and some of the nodes did not have an assigned IP address, Claudie would fail to correctly determine if the nodes were reachable. [#1691](https://github.com/berops/claudie/pull/1691)
- Claudie will now increase the limits for `fs.inotify` to a higher number, as depending on the workload on each node, reaching the limits would result in an error from which Claudie would not recover. [#1696](https://github.com/berops/claudie/pull/1696) 
- Annotations for static nodepools will now be correctly propagated. [#1696](https://github.com/berops/claudie/pull/1696) 

## v0.9.9

## What's Changed
- General maintenance release, updated dependencies used by Claudie [#1709](https://github.com/berops/claudie/pull/1709)

- Upgrading Longhorn from version 1.7.0 to version 1.8.1 [#1709](https://github.com/berops/claudie/pull/1709)
  
  After upgrading Longhorn to the newer version, some pods of the old and new versions will coexist if your cluster uses a PVC that uses the Longhorn storage class (which is the default), as they would reference the old v1.7.0.
  
  To upgrade the volumes to the newer version, it's possible to use the Longhorn UI to set `Settings > Concurrent Automatic Engine Upgrade Per Node Limit` to a value greater than 0 to upgrade old volumes.
  This is a setting that controls how Longhorn automatically upgrades volumes’ engines to the new default engine image after upgrading Longhorn manager. More on: https://longhorn.io/docs/1.8.1/deploy/upgrade/auto-upgrade-engine/
  
  Once the upgrade is complete, the old engine image pods and the instance manager will be terminated after ~60 minutes of non-use (after all volumes have been upgraded to use the latest Longhorn version) You can also follow the official Longhorn post on this: https://longhorn.io/kb/troubleshooting-some-old-instance-manager-pods-are-still-running-after-upgrade/

## v0.9.10

## What's Changed

- Decrease the amount of retries for cleanup of static nodes during deletion from 4 to 2 [#1729](https://github.com/berops/claudie/pull/1729)

## Bug fixes

- Fix panic when deleting clusters with static nodes for which DNS was not built correctly [#1724](https://github.com/berops/claudie/pull/1724)
- Fix propagation of desired state from operator to manager service [#1726](https://github.com/berops/claudie/pull/1726)
- Fix multiple HTTP proxy environment variables present in `/etc/environment` [#1727](https://github.com/berops/claudie/pull/1727)
- Fix partial DNS apply, which would left part of the infrastructure untracked [#1728](https://github.com/berops/claudie/pull/1728)

## v0.9.11

## What's Changed
**READ ME: A lot of core changes are made in this release, before updating an already deployed Claudie instance, make sure you have working backups of your kuberentes clusters**

- InputManifest was extended to also include a NoProxy list in the proxy settings to bypass the proxy for the listed endpoints, if used. [#1745](https://github.com/berops/claudie/pull/1745)
```
kubernetes:
    clusters:
      - name: proxy-example
        version: "1.30.0"
        network: 192.168.2.0/24
        installationProxy:
            mode: "on"
            noProxy: ".suse.com"
```

- Update kubeone to 1.10 [#1749](https://github.com/berops/claudie/pull/1749)
- Migrate to OpenTofu `v1.6.2` from terraform `v1.5.7` [#1755](https://github.com/berops/claudie/pull/1755)

  **READ ME: OpenTofu 1.6.2 is compatible with the previosly used Terraform version 1.5.7, while claudie will take care of the update, make sure you have working backups if you are updating an already deployed Claudie instance, in case of a disaster scenario**

- Add `sprig` to all templates used within claudie [#1768](https://github.com/berops/claudie/pull/1768)
- Builder will now support faster termination and wait only on the current task being processed instead of the whole workflow [#1770](https://github.com/berops/claudie/pull/1770)

- Claudie will now support proper HA DNS Loadbalancing #[1777](https://github.com/berops/claudie/pull/1777)
  
  **This feature will be available with the latest claudie templates [`v0.9.11`](https://github.com/berops/claudie-config/releases/tag/v0.9.11)**
  
  **READ ME: for already deployed Claudie instances, if you used Cloudflare as a provider you will need to update your secret to also include the [Accound ID](https://docs.claudie.io/v0.9.11/input-manifest/providers/cloudflare/) the token was created for.**
  
- NGINX was replaced by Envoy on Loadbalancers. https://github.com/berops/claudie/pull/1735
  
  **READ ME: If you update an already deployed Claudie instance, this is a one time update that will introduce a small downtime of the services while NGINX is being replaced with Envoy.**
  
- Upgraded all terraform providers to the latest possible version that still supports the claudie templates version `v0.9.8` [#1782](https://github.com/berops/claudie/pull/1782)



- Claudie will now perform a rollout restart for the NVIDIA GPU operator daemonset as part of the workflow, which overwrites the `/etc/containerd/config.yml`. [#1790](https://github.com/berops/claudie/pull/1790)

## Bug fixes
- Return partially updated state instead of always defaulting to current state after error in deletion [#1793](https://github.com/berops/claudie/pull/1793)
- Restarting SSH session after updating environmnet variables, is now part of the ansible workflow, which previosly caused issue in which the updated environment variables were not reflected in a re-used SSH connection [#1792](https://github.com/berops/claudie/pull/1792)
- Fixed a memory leak in the autoscaler service. [#1787](https://github.com/berops/claudie/pull/1787)

## v0.9.12

## What's Changed
- Retries were added to reading the output from OpenTofu, which could occasionally fail. [#1824](https://github.com/berops/claudie/pull/1824)
- Increased concurrency limits to decrease the build time of larger clusters. This change also affects Claudie's memory requirements, which should fit within 8 GB. [#1819](https://github.com/berops/claudie/pull/1819)
- For autoscaled events, Terraformer will now skip refreshing the LoadBalancers and DNS infrastructure, if present. [#1830](https://github.com/berops/claudie/pull/1830)

## v0.9.13

## What's Changed
- Concurrency limits are now configurable [#1838](https://github.com/berops/claudie/pull/1838)
- Autoscaled nodepools are now limited to 256 nodes [#1839](https://github.com/berops/claudie/pull/1839)
- Metadata secret will now be updated after node deletion [#1841](https://github.com/berops/claudie/pull/1841)
- Builder TTL has been increased to 42 hours, as 2 hours is a small amount of time to react to issues [#1841](https://github.com/berops/claudie/pull/1850)

## Bug fixes
- Prometheus metric for currently deleted nodes has been fixed [#1849](https://github.com/berops/claudie/pull/1849)
