# Claudie Hardening

In this section we'll describe how to further configure security hardening of the default
deployment for claudie.

## Passwords

When deploying the default manifests claudie uses simple passwords for MongoDB, DynamoDB
and MinIO.

You can find the passwords at these paths:

```
manifests/claudie/mongo/secrets
manifests/claudie/minio/secrets
manifests/claudie/dynamo/secrets
```

It is highly recommended that you change these passwords to more secure ones.

## Network Policies

The default deployment of claudie comes without any network policies, as based on the
CNI on the Management cluster the network policies may not be fully supported.

We have a set of network policies pre-defined that can be found in:

```
manifests/network-policies
```

Currently, we have a cilium specific network policy that's using `CiliumNetworkPolicy` and another that 
uses `NetworkPolicy` which should be supported by most network plugins.

To install network policies you can simply execute one the following commands:

```
# for clusters using cilium as their CNI
kubectl apply -f https://github.com/berops/claudie/releases/latest/download/network-policy-cilium.yaml
```

```
# other
kubectl apply -f https://github.com/berops/claudie/releases/latest/download/network-policy.yaml
```