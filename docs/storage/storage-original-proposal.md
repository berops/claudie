# Claudie storage proposal

## Concept

Ability to run stateful workloads is a must. At the same time running stateful workloads is complex. Here the complexity is on another level considering the multi-/hybrid-cloud environment. Therefore, Claudie needs to be able to accommodate the stateful workloads, regardless of the underlying infrastructure providers.

Orchestrate storage on the kubernetes cluster nodes by creating one storage cluster across multiple providers. This storage cluster will have a series of `zones`, one for each cloud provider. Each `zone` should store its own persistent volume data.

## Solutions to consider

- ceph + rook
- longhorn
- glusterFS
- storageOS
- chubaoFS
- openEBS

## Note

Explore additional strategies if the ones above turn out to be inappropriate/infeasible
- create a storage cluster in each cloud provider and mirror the data between all storage clusters
- create one storage cluster located in one cloud provider
  - machines on other providers will pull data from this cluster
