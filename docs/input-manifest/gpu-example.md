We will follow the guide
from [Nvidia](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/getting-started.html#operator-install-guide)
to deploy the `gpu-operator` into a claudie build kubernetes cluster. Make sure you fulfill the necessary listed
requirements in prerequisites before continuing, if you decide to use a different cloud provider.

In this example we will be using [GenesisCloud](providers/genesiscloud.md) as our provider, with the following config:

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
      templates:
        repository: "https://github.com/berops/claudie-config"
        tag: "v0.9.8"
        path: "templates/terraformer/genesiscloud"
      secretRef:
        name: genesiscloud-secret
        namespace: secrets

  nodePools:
    dynamic:
    - name: gencloud-cpu
      providerSpec:
        name: genesiscloud
        region: ARC-IS-HAF-1
      count: 1
      serverType: vcpu-2_memory-4g_disk-80g
      image: "Ubuntu 22.04"
      storageDiskSize: 50

    - name: gencloud-gpu
      providerSpec:
        name: genesiscloud
        region: ARC-IS-HAF-1
      count: 2
      serverType: vcpu-4_memory-12g_disk-80g_nvidia3080-1
      image: "Ubuntu 22.04"
      storageDiskSize: 50

  kubernetes:
    clusters:
      - name: gpu-example
        version: v1.31.0
        network: 172.16.2.0/24
        pools:
          control:
            - gencloud-cpu
          compute:
            - gencloud-gpu
```

After the `InputManifest` was successfully build by claudie, we deploy the `gpu-operator` to the `gpu-examepl`kubernetes cluster.

1. Create a namespace for the gpu-operator.

```bash
kubectl create ns gpu-operator
```

```bash
kubectl label --overwrite ns gpu-operator pod-security.kubernetes.io/enforce=privileged
```

2. Add Nvidia Helm repository.

```bash
helm repo add nvidia https://helm.ngc.nvidia.com/nvidia \
    && helm repo update
```

3. Install the operator.

```bash
helm install --wait --generate-name \
    -n gpu-operator --create-namespace \
    nvidia/gpu-operator
```

4. Wait for the pods in the `gpu-operator` namespace to be ready.

```bash
NAME                                                              READY   STATUS      RESTARTS      AGE
gpu-feature-discovery-4lrbz                                       1/1     Running     0              10m
gpu-feature-discovery-5x88d                                       1/1     Running     0              10m
gpu-operator-1708080094-node-feature-discovery-gc-84ff8f47tn7cd   1/1     Running     0              10m
gpu-operator-1708080094-node-feature-discovery-master-757c27tm6   1/1     Running     0              10m
gpu-operator-1708080094-node-feature-discovery-worker-495z2       1/1     Running     0              10m
gpu-operator-1708080094-node-feature-discovery-worker-n8fl6       1/1     Running     0              10m
gpu-operator-1708080094-node-feature-discovery-worker-znsk4       1/1     Running     0              10m
gpu-operator-6dfb9bd487-2gxzr                                     1/1     Running     0              10m
nvidia-container-toolkit-daemonset-jnqwn                          1/1     Running     0              10m
nvidia-container-toolkit-daemonset-x9t56                          1/1     Running     0              10m
nvidia-cuda-validator-l4w85                                       0/1     Completed   0              10m
nvidia-cuda-validator-lqxhq                                       0/1     Completed   0              10m
nvidia-dcgm-exporter-l9nzt                                        1/1     Running     0              10m
nvidia-dcgm-exporter-q7c2x                                        1/1     Running     0              10m
nvidia-device-plugin-daemonset-dbjjl                              1/1     Running     0              10m
nvidia-device-plugin-daemonset-x5kfs                              1/1     Running     0              10m
nvidia-driver-daemonset-dcq4g                                     1/1     Running     0              10m
nvidia-driver-daemonset-sjjlb                                     1/1     Running     0              10m
nvidia-operator-validator-jbc7r                                   1/1     Running     0              10m
nvidia-operator-validator-q59mc                                   1/1     Running     0              10m
```

When all pods are ready you should be able to verify if the GPUs can be used

```bash
kubectl get nodes -o json | jq -r '.items[] | {name:.metadata.name, gpus:.status.capacity."nvidia.com/gpu"}'
```

5. Deploy an example manifest that uses one of the available GPUs from the worker nodes.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: cuda-vectoradd
spec:
  restartPolicy: OnFailure
  containers:
    - name: cuda-vectoradd
      image: "nvcr.io/nvidia/k8s/cuda-sample:vectoradd-cuda11.7.1-ubuntu20.04"
      resources:
        limits:
          nvidia.com/gpu: 1
```

From the logs of the pods you should be able to see

```bash
kubectl logs cuda-vectoradd
[Vector addition of 50000 elements]
Copy input data from the host memory to the CUDA device
CUDA kernel launch with 196 blocks of 256 threads
Copy output data from the CUDA device to the host memory
Test PASSED
Done
```
