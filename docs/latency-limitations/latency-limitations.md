# Latency-imposed limitations

## etcd limitations

A distance between etcd nodes in the multi-cloud environment of more than 600 km can be detrimental to cluster health. In a scenario like this, an average deployment time can double compared to a scenario with etcd nodes in different availability zones within the same cloud provider. Besides this, the total number of the etcd Slow Applies increases rapidly, and a Round-trip time varies from ~0.05s to ~0.2s, whereas in a single-cloud scenario with etcd nodes in a different AZs the range is from ~0.003s to ~0.025s. 

In multi-cloud clusters, a request to a KubeAPI lasts from ~0.025s to ~0.25s. On the other hand, in a one-cloud scenario, they last from ~0.005s to ~0.025s.

You can read more about this topic [here](https://www.berops.com/blog/evaluating-etcds-performance-in-multi-cloud).

## Longhorn limitations

There are basically these three problems when dealing with a high latency in Longhorn:

* Kubelet fails to mount the RWO or RWX volume to a workload pod in case the latency between the node hosting the pod and the nodes with the replicas is greater than ~100ms.
* Some replicas of a volume might not catch up if the latency between nodes that host replicas is greater than ~100ms.
* In case of RWX volumes, Longhorn spawns a `share-manager` pod that hosts the NFS server to facilitate the data export to the workload pods. If the latency between the node with a `share-manager` pod and the node with a workload pod is greater than ~100ms, kubelet fails to mount the volume to the workload pod.

Generally, a single volume with 3 replicas can tolerate a maximum network latency of around 100ms. In the case of a multiple-volume scenario, the maximum network latency can be no more than 20ms. The network latency has a significant impact on IO performance and total network bandwidth. See more about CPU and network requirements [here](https://github.com/longhorn/longhorn/issues/1691#issuecomment-729633995)

### How to avoid high latency problems

When dealing with RWO volumes you can avoid mount failures caused by high latency by setting Longhorn to only use storage on specific nodes (follow this [tutorial](https://longhorn.io/kb/tip-only-use-storage-on-a-set-of-nodes/)) and using [nodeAffinity](https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/) or [nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector) to schedule your workload pods only to the nodes that have replicas of the volumes or are close to them.

### How to mitigate high latency problems with RWX volumes

To mitigate high latency issues with RWX volumes you can maximize these Longhorn settings:

* [Engine Replica Timeout](https://longhorn.io/docs/1.6.0/references/settings/#engine-to-replica-timeout) - max 30s
* [Replica File Sync HTTP Timeout](https://longhorn.io/docs/1.6.0/references/settings/#timeout-of-http-client-to-replica-file-sync-server) - max 120s
* [Guaranteed Instance Manager CPU](https://longhorn.io/docs/1.6.0/references/settings/#guaranteed-instance-manager-cpu) - max 40%

Thanks to maximizing these settings you should successfully mount a RWX volume for which a latency between a node with a `share-manager` pod and a node with a workload pod + replica is ~200ms. However, it will take from 7 to 10 minutes. Also, there are some resource requirements on the nodes and limitations on the maximum size of the RWX volumes. For example, you will not succeed in mounting even a 1Gi RWX volume for which a latency between a node with a `share-manager` pod and a node with a workload pod + replica is ~200ms, if the nodes have only 2 shared vCPUs and 4GB RAM. This applies even when there are no other workloads in the cluster. Your nodes need at least 2vCPU and 8GB RAM. Generally, the more CPU you assign to the Longhorn manager the more you can mitigate the issue with high latency and RWX volumes.

Keep in mind, that using machines with higher resources and maximizing these Longhorn settings doesn't necessarily guarantee successful mount of the RWX volumes. It also depends on the size of these volumes. For example, even after maximizing these settings and using nodes with 2vCPU and 8GB RAM with ~200ms latency between them, you will fail to mount a 10Gi volume to the workload pod in case you try to mount multiple volumes at once. In case you do it one by one, you should be good. 

To conclude, maximizing these Longhorn settings can help to mitigate the high latency issue when mounting RWX volumes, but it is resource-hungry and it also depends on the size of the RWX volume + the total number of the RWX volumes that are attaching at once.
