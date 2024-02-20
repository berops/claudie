# Latency-imposed limitations

## etcd limitations

A distance between etcd nodes in the multi-cloud environment to more than 600 km can be detrimental for the cluster health. An average deployment time can double compared to etcd nodes in a different zone within the same cloud provider. Besides this a total number of etcd Slow Applies increases rapidly.

Round-trip time from ... to ... increase about %

In a multi-cloud clusters a request to a KubeAPI last from 0.025s to 0.25s. In a one cloud scenario they last from 0.005s to 0.025s

## Longhorn limitations

### Replicas locations

### ReadWritMany
