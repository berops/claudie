# Latency-imposed limitations

## etcd limitations

A distance between etcd nodes in the multi-cloud environment to more than 600 km can be detrimental for the cluster health. An average deployment time can double compared to etcd nodes in a different zone within the same cloud provider. Besides this a total number of etcd Slow Applies increases rapidly.

Round-trip time from ... to ... increase about %

In a multi-cloud clusters a request to a KubeAPI last from 0.025s to 0.25s. In a one cloud scenario they last from 0.005s to 0.025s

## Longhorn limitations

There are basically three types of errors you can bump into when dealing with a high latency in Longhorn:

* kubelet fails to mount the volume to your workload pod in case the latency is ~100ms or higher. It can also fail the first mount try, when the latency is ~50ms, however it can eventually succeed after a restart of the workload pod.
* if the latency between your nodes that host replicas of your volume is greater than ~100ms some of the replicas might not catch up.
* in case of using RWX volumes Longhorn spaws a `share-manager` pod that hosts NFS server to facilitate the data export to the workload pods. If the latency between the node with a `share-manager` pod and nodes that has replicas of the volume is greater than ~100ms you will most probably bump into an the issue mentioned in the first point.

A single volume with 3 replicas can tolerate maximum network latency around 100ms. In case of multiple-volume scenario, the maximum network latency can be no more than 20ms.

The network latency has a significant impact on IO performance and total network bandwidth.

See more [here](https://github.com/longhorn/longhorn/issues/1691#issuecomment-729633995)

### How to avoid high latency in Longhorn

When using RWO volumes you can avoid high latency issues by setting Longhorn to only use storage on a specific nodes (follow this [tutorial](https://longhorn.io/kb/tip-only-use-storage-on-a-set-of-nodes/)) and using [nodeAffinity](https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/) or [nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector) to schedule your workload pods only to nodes that has replicas of the volume or are really close to them.

### How to mitigate high latency problems with RWX volumes

To mitigate high latency issues with RWX volumes you can maximize these Longhorn settings:

* [Engine Replica Timeout](https://longhorn.io/docs/1.6.0/references/settings/#engine-to-replica-timeout) - max 30s
* [Replica File Sync HTTP Timeout](https://longhorn.io/docs/1.6.0/references/settings/#timeout-of-http-client-to-replica-file-sync-server) - max 120s
* [Guaranteed Instance Manager CPU](https://longhorn.io/docs/1.6.0/references/settings/#guaranteed-instance-manager-cpu) - max 40%

Thanks to maximizing these settings you will mount a RWX volume that has ~200ms latency between a node with a `share-manager` pod and a node with a workload pod, but it will take from 7 to 10 minutes. However, there are some requirements on these k8s nodes and the maximum size of the volumes. For example, you will not succeed in mounting a RWX volume with a ~200ms latency between node with `share-manager` pod and a node with a workload pod, if your nodes has (2vCPU shared and 4GB RAM). This applies even when there are no other wokloads. You need at least 2vCPU and 8GB RAM. Generally the more CPU you assing to Longhorn manager the more you mitigate issue with high latency and RWX volumes.

Keep in mind, that using machines with higher resources and maximizing these Longhorn settings doesn't necessarily guarantee successful mounting. Successful mount also depends on the size of the RWX volume. For example, even after maximizing these Longhorn settings and using nodes with 2vCPU and 8GB RAM with latency ~200ms you will fail to mount 10Gi volume to the workload pod in case you will try to mount multiple at once. In case you do it one by one, you should be good. 

To conclude this, maximizing the these Longhorn settings can help to mitigate the high latency issue when mounting RWX volumes, but it is really resource-hungry and it also depends on the size of the RWX volume + the total number of the RWX volumes that are attaching at once.
