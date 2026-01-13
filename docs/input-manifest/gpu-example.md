We will follow the guide
from [Nvidia](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/getting-started.html#operator-install-guide)
to deploy the `gpu-operator` into a Claudie-built Kubernetes cluster. Make sure you fulfill the necessary listed
requirements in prerequisites before continuing, if you decide to use a different cloud provider.

In this example we will be using [AWS](providers/aws.md) as our provider, with the following config:

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: aws-gpu-example
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: aws-1
      providerType: aws
      secretRef:
        name: aws-secret
        namespace: secrets

  nodePools:
    dynamic:
    - name: control-aws
      providerSpec:
        name: aws-1
        region: eu-central-1
        zone: eu-central-1a
      count: 1
      serverType: t3.medium
      # AMI ID of the image Ubuntu 24.04.
      # Make sure to update it according to the region.
      image: ami-07eef52105e8a2059

    - name: gpu-aws
      providerSpec:
        name: aws-1
        region: eu-central-1
        zone: eu-central-1a
      count: 2
      serverType: g4dn.xlarge
      # AMI ID of the image Ubuntu 24.04.
      # Make sure to update it according to the region.
      image: ami-07eef52105e8a2059
      storageDiskSize: 50

  kubernetes:
    clusters:
      - name: gpu-example
        version: v1.31.0
        network: 172.16.2.0/24
        pools:
          control:
            - control-aws
          compute:
            - gpu-aws
```

After the `InputManifest` has been successfully built by Claudie, deploy the `gpu-operator` to the `gpu-example` Kubernetes cluster.

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

When all pods are ready, you should be able to verify if the GPUs can be used.

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
