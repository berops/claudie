# Genesis Cloud
Genesis cloud provider requires `apitoken` token field in string format.

## Compute example
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: genesiscloud-secret
data:
  apitoken: GCAAAZZZZnnnnNNNNxXXX123BBcc123qqcva
type: Opaque
```

## Create Genesis Cloud API token
You can create Genesis Cloud API token by following [this guide](https://support.genesiscloud.com/support/solutions/articles/47001126146-how-to-generate-an-api-token-). The token must be able to have access to the following compute resources.

```
Instances, Network, Volumes
```

## Input manifest examples

### Single provider, multi region cluster example

#### Create a secret for Genesis cloud provider
```bash
kubectl create secret generic genesiscloud-secret --namespace=mynamespace --from-literal=apitoken='GCAAAZZZZnnnnNNNNxXXX123BBcc123qqcva'
```

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: genesis-example
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: genesiscloud
      providerType: genesiscloud
      secretRef:
        name: genesiscloud-secret
        namespace: mynamespace

  nodePools:
    dynamic:
      - name: control
        providerSpec:
          name: genesiscloud
          region: ARC-IS-HAF-1
        count: 1
        serverType: vcpu-2_memory-4g_disk-80g
        image: "Ubuntu 22.04"
        storageDiskSize: 50

      - name: compute
        providerSpec:
          name: genesiscloud
          region: ARC-IS-HAF-1
        count: 3
        serverType: vcpu-2_memory-4g_disk-80g
        image: "Ubuntu 22.04"
        storageDiskSize: 50

  kubernetes:
    clusters:
      - name: genesiscloud-cluster
        version: v1.31.0
        network: 172.16.2.0/24
        pools:
          control:
            - control
          compute:
            - compute
```
