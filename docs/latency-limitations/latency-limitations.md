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
