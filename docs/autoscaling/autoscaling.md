# Autoscaling in Claudie

Claudie supports autoscaling by installing [Cluster Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler) for Claudie-made clusters, with a custom implementation of `external gRPC cloud provider`, in Claudie context called `autoscaler-adapter`. This, together with Cluster Autoscaler is automatically managed by Claudie, for any clusters, which have at least one node pool defined with `autoscaler` field. Whats more, you can change the node pool specification freely from autoscaler configuration to static count or vice versa. Claudie will seamlessly configure Cluster Autoscaler, or even remove it when it is no longer needed.

## What triggers a scale up

The scale up is triggered if there are pods in the cluster, which are unschedulable and

- could be scheduled, if any of the node pools with autoscaling enabled would accommodate them if they would grow in size
- the node pools, which could accommodate them, are not yet at maximum size

However, if pods' resource requests are larger than any new node would offer, the scale up will not be triggered. The cluster is scanned every 10 seconds for these pods, to assure quick response to the cluster needs. For more information, please have a look at [official Cluster Autoscaler documentation](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#how-does-scale-up-work).

## What triggers a scale down

The scale down is triggered, if all following conditions are met

- the sum of CPU and memory requests of all pods running on node considered for scale down is below 50% (Claudie by default excludes DaemonSet pods and Mirror pods)
- all pods running on the node (except those that run on all nodes by default, like manifest-run pods or pods created by DaemonSets) considered for scale down,  can be scheduled to other nodes
- the node considered for scale down does not have [scale-down disabled annotation](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#how-can-i-prevent-cluster-autoscaler-from-scaling-down-a-particular-node)

For more information, please have a look at [official Cluster Autoscaler documentation](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#how-does-scale-down-work).

## Architecture

As stated earlier, Claudie deploys Cluster Autoscaler and Autoscaler Adapter for every Claudie-made cluster which enables it. These components are deployed within the same cluster as Claudie.

![autoscaling-architecture](autoscaling.jpg)

## Considerations

As Claudie just extends Cluster Autoscaler, it is important that you follow their [best practices](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#what-are-the-key-best-practices-for-running-cluster-autoscaler). Furthermore, as number of nodes in autoscaled node pools can be volatile, you should carefully plan out how you will use the storage on such node pools. Longhorn support of Cluster Autoscaler is still in experimental phase ([longhorn documentation](https://longhorn.io/docs/1.4.0/high-availability/k8s-cluster-autoscaler/)).
