# On premise nodes

Claudie is designed to leverage your existing infrastructure and utilise it for building Kubernetes clusters together with supported cloud providers. However, Claudie operates under a few assumptions:

1. Accessibility of Machines: Claudie requires access to the machines specified by the provided endpoint. It needs the ability to connect to these machines in order to perform necessary operations.

2. Connectivity between Static Nodes: Static nodes within the infrastructure should be able to communicate with each other using the specified endpoints. This connectivity is important for proper functioning of the Kubernetes cluster.

3. SSH Access with Root Privileges: Claudie relies on SSH access to the nodes using the SSH key provided in the input manifest. The SSH key should grant root privileges to enable Claudie to perform required operations on the nodes.

4. Meeting the Kubernetes nodes requirements: Learn [more](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/#before-you-begin).

By ensuring that these assumptions are met, Claudie can effectively utilise your infrastructure and build Kubernetes clusters while collaborating with the supported cloud providers.

## Private key example secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: static-node-key
data:
  privatekey: <base64-encoded-private-key>
type: Opaque
```

## Input manifest example

### Private cluster example

```bash
kubectl create secret generic static-node-key --namespace=<your-namespace> --from-file=privatekey=private.pem
```

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: private-cluster-example
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
                namespace: <your-namespace>

        - name: compute
          nodes:
            - endpoint: "192.168.10.2"
              secretRef:
                name: static-node-key
                namespace: <your-namespace>
            - endpoint: "192.168.10.3"
              secretRef:
                name: static-node-key
                namespace: <your-namespace>

  kubernetes:
    clusters:
      - name: private-cluster
        version: 1.27.0
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
kubectl create secret generic static-node-key --namespace=<your-namespace> --from-file=privatekey=private.pem
```

> To see how to configure Hetzner or any other credentials for hybrid cloud, refer to their docs.

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: hybrid-cloud-example
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: hetzner-1
      providerType: hetzner
      secretRef:
        name: hetzner-secret-1
        namespace: <your-namespace>

  nodePools:
    dynamic:
      - name: control-htz
        providerSpec:
          name: hetzner-1
          region: fsn1
          zone: fsn1-dc14
        count: 3
        serverType: cpx22
        image: ubuntu-24.04

    static:
        - name: datacenter-1
          nodes:
            - endpoint: "192.168.10.1"
              secretRef:
                name: static-node-key
                namespace: <your-namespace>
            - endpoint: "192.168.10.2"
              secretRef:
                name: static-node-key
                namespace: <your-namespace>
            - endpoint: "192.168.10.3"
              secretRef:
                name: static-node-key
                namespace: <your-namespace>

  kubernetes:
    clusters:
      - name: hybrid-cluster
        version: 1.27.0
        network: 192.168.2.0/24
        pools:
          control:
            - control-hetzner
          compute:
            - datacenter-1
```
