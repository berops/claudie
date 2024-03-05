# On premise nodes

Claudie is designed to leverage your existing infrastructure and utilise it for building Kubernetes clusters together with supported cloud providers. However, Claudie operates under a few assumptions:

1. Accessibility of Machines: Claudie requires access to the machines specified by the provided endpoint. It needs the ability to connect to these machines in order to perform necessary operations.

2. Connectivity between Static Nodes: Static nodes within the infrastructure should be able to communicate with each other using the specified endpoints. This connectivity is important for proper functioning of the Kubernetes cluster.

3. SSH Access with Root Privileges: Claudie relies on SSH access to the nodes using the SSH key provided in the input manifest. The SSH key should grant root privileges to enable Claudie to perform required operations on the nodes.

By ensuring that these assumptions are met, Claudie can effectively utilise your infrastructure and build Kubernetes clusters while collaborating with the supported cloud providers.

## Private key example secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: static-node-key
data:
  privatekey: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcFFJQkFBS0NBUUVBbzJEOGNYb0Uxb3VDblBYcXFpVW5qbHh0c1A4YXlKQW4zeFhYdmxLOTMwcDZBUzZMCncvVW03THFnbUhpOW9GL3pWVnB0TDhZNmE2NWUvWjk0dE9SQ0lHY0VJendpQXF3M3M4NGVNcnoyQXlrSWhsWE0KVEpSS3J3SHJrbDRtVlBvdE9paDFtZkVTenFMZ25TMWdmQWZxSUVNVFdOZlRkQmhtUXpBNVJFT2NpQ1Q1dFRnMApraDI1SmVHeU9qR3pzaFhkKzdaVi9PUXVQUk5Mb2lrQzFDVFdtM0FSVFFDeUpZaXR5bURVeEgwa09wa2VyODVoCmpFRTRkUnUxVzQ2WDZkdEUrSlBZNkNKRlR2c1VUcGlqT3QzQmNTSTYyY2ZyYmFRYXhvQXk2bEJLVlB1cm1xYm0Kb09JNHVRUWJWRGt5Q3V4MzcwSTFjTUVzWkszYVNBa0ZZSUlMRndJREFRQUJBb0lCQUVLUzFhc2p6bTdpSUZIMwpQeTBmd0xPWTVEVzRiZUNHSlVrWkxIVm9YK2hwLzdjVmtXeERMQjVRbWZvblVSWFZvMkVIWFBDWHROeUdERDBLCnkzUGlnek9TNXJPNDRCNzRzQ1g3ZW9Dd1VRck9vS09rdUlBSCtUckE3STRUQVVtbE8rS3o4OS9MeFI4Z2JhaCsKZ2c5b1pqWEpQMHYzZmptVGE3QTdLVXF3eGtzUEpORFhyN0J2MkhGc3ZueHROTkhWV3JBcjA3NUpSU2U3akJIRgpyQnpIRGFOUUhjYWwybTJWbDAvbGM4SVgyOEIwSXBYOEM5ajNqVGUwRS9XOVYyaURvM0ZvbmZzVU1BSm9KeW1nCkRzRXFxb25Cc0ZFeE9iY1BUNlh4SHRLVHVXMkRDRHF3c20xTVM2L0xUZzRtMFZ0alBRbGE5cnd0Z1lQcEtVSWYKbkRya3ZBRUNnWUVBOC9EUTRtNWF4UE0xL2d4UmVFNVZJSEMzRjVNK0s0S0dsdUNTVUNHcmtlNnpyVmhOZXllMwplbWpUV21lUmQ4L0szYzVxeGhJeGkvWE8vc0ZvREthSjdHaVl4L2RiOEl6dlJZYkw2ZHJiOVh0aHVObmhJWTlkCmJPd0VhbWxXZGxZbzlhUTBoYTFpSHpoUHVhMjN0TUNiM2xpZzE3MVZuUURhTXlhS3plaVMxUmNDZ1lFQXEzU2YKVEozcDRucmh4VjJiMEJKUStEdjkrRHNiZFBCY0pPbHpYVVVodHB6d3JyT3VKdzRUUXFXeG1pZTlhK1lpSzd0cAplY2YyOEltdHY0dy9aazg1TUdmQm9hTkpwdUNmNWxoMElseDB3ZXROQXlmb3dTNHZ3dUlZNG1zVFlvcE1WV20yClV5QzlqQ1M4Q0Y2Y1FrUVdjaVVlc2dVWHFocE50bXNLTG9LWU9nRUNnWUVBNWVwZVpsd09qenlQOGY4WU5tVFcKRlBwSGh4L1BZK0RsQzRWa1FjUktXZ1A2TTNKYnJLelZZTGsySXlva1VDRjRHakI0TUhGclkzZnRmZTA2TFZvMQorcXptK3Vub0xNUVlySllNMFQvbk91cnNRdmFRR3pwdG1zQ2t0TXJOcEVFMjM3YkJqaERKdjVVcWgxMzFISmJCCkVnTEVyaklVWkNNdWhURlplQk14ZVVjQ2dZRUFqZkZPc0M5TG9hUDVwVnVKMHdoVzRDdEtabWNJcEJjWk1iWFQKUERRdlpPOG9rbmxPaENheTYwb2hibTNYODZ2aVBqSTVjQWlMOXpjRUVNQWEvS2c1d0VrbGxKdUtMZzFvVTFxSApTcXNnUGlwKzUwM3k4M3M1THkzZlRCTTVTU3NWWnVETmdLUnFSOHRobjh3enNPaU5iSkl1aDFLUDlOTXg0d05hCnVvYURZQUVDZ1lFQW5xNzJJUEU1MlFwekpjSDU5RmRpbS8zOU1KYU1HZlhZZkJBNXJoenZnMmc5TW9URXpWKysKSVZ2SDFTSjdNTTB1SVBCa1FpbC91V083bU9DR2hHVHV3TGt3Uy9JU1FjTmRhSHlTRDNiZzdndzc5aG1UTVhiMgozVFpCTjdtb3FWM0VhRUhWVU1nT1N3dHUySTlQN1RJNGJJV0RQUWxuWE53Q0tCWWNKanRraWNRPQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
type: Opaque
```

## Input manifest example

### Private cluster example

```bash
kubectl create secret generic static-node-key --namespace=mynamespace --from-file=privatekey=private.pem
```

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: PrivateClusterExample
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  nodePools:
    static:
        - name: control
          nodes:
            - endpoint: "192.168.10.1"
              secretRef:
                name: static-node-key
                namespace: mynamespace

          - name: compute
            - endpoint: "192.168.10.2"
              secretRef:
                name: static-node-key
                namespace: mynamespace
            - endpoint: "192.168.10.3"
              secretRef:
                name: static-node-key
                namespace: mynamespace

  kubernetes:
    clusters:
      - name: private-cluster
        version: v1.26.13
        network: 192.168.2.0/24
        pools:
          control:
            - control
          compute:
            - compute
```

### Hybrid cloud example

### Create secret for private key

```bash
kubectl create secret generic static-node-key --namespace=mynamespace --from-file=privatekey=private.pem
```

> To see how to configure Hetzner or any other credentials for hybrid cloud, refer to their docs.

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: HybridCloudExample
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: hetzner-1
      providerType: hetzner
      secretRef:
        name: hetzner-secret-1
        namespace: mynamespace

  nodePools:
    dynamic:
      - name: control-hetzner
        providerSpec:
          name: hetzner-1
          region: fsn1
          zone: fsn1-dc14
        count: 3
        serverType: cpx11
        image: ubuntu-22.04

    static:
        - name: datacenter-1
          nodes:
            - endpoint: "192.168.10.1"
              secretRef:
                name: static-node-key
                namespace: mynamespace
            - endpoint: "192.168.10.2"
              secretRef:
                name: static-node-key
                namespace: mynamespace
            - endpoint: "192.168.10.3"
              secretRef:
                name: static-node-key
                namespace: mynamespace

  kubernetes:
    clusters:
      - name: hybrid-cluster
        version: v1.26.13
        network: 192.168.2.0/24
        pools:
          control:
            - control-hetzner
          compute:
            - datacenter-1
```
