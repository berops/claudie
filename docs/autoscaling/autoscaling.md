# Autoscaling in Claudie

Claudie supports autoscaling by installing [Cluster Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler) for Claudie-made clusters, with a custom implementation of `external gRPC cloud provider`, in Claudie context called `autoscaler-adapter`. This, together with Cluster Autoscaler is automatically managed by Claudie, for any clusters, which have at least one node pool defined with `autoscalerConf`. Whats more, you can change the nodepool specification freely, from autoscaler configuration to static count or vice versa. Claudie will seamlessly configure Cluster Autoscaler, or even remove it when its no longer needed.

## Architecture

As stated earlier, Claudie deploys Cluster Autoscaler and Autoscaler Adapter for every Claudie-made cluster who enables it. These components are deployed within the same cluster as Claudie is running in.

![autoscaling-architecture](autoscaling.jpg)

## Considerations

As Claudie just extends Cluster Autoscaler, it is important that you follow their [best practices](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#what-are-the-key-best-practices-for-running-cluster-autoscaler). Furthermore, as number of nodes in autoscaled node pools can be volatile, you should carefully plan out how you will use the storage on such node pools. Longhorn support of Cluster Autoscaler is still in experimental phase ([longhorn documentation](https://longhorn.io/docs/1.4.0/high-availability/k8s-cluster-autoscaler/)).
