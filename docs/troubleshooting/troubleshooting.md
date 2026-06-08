# Troubleshooting guide

!!! note "In progress"
    As we continue expanding our troubleshooting guide, we understand that issues may arise during your usage of Claudie. Although the guide is not yet complete, we encourage you to create a [GitHub issue](https://github.com/berops/claudie/issues) if you encounter any problems. Your feedback and reports are highly valuable to us in improving our platform and addressing any issues you may face.

## Claudie cluster not starting
Claudie relies on all services to be interconnected. If any of these services fail to create due to node unavailability or resource constraints, Claudie will be unable to provision your cluster.

1. Check if all Claudie services are running:

    ```bash
    kubectl get pods -n claudie
    ```

    ```text
    NAME                                READY   STATUS              RESTARTS       AGE
    ansibler-6bf78cccf4-pnxrk           1/1     Running             0              3m20s
    claudie-operator-64c9554c66-rvtr5   1/1     Running             0              3m19s
    kube-eleven-7bd47945c5-kbpd6        1/1     Running             0              3m19s
    kuber-64554ffffc-fkdj6              1/1     Running             0              3m19s
    make-bucket-job-4mxw7               0/1     Completed           0              3m19s
    manager-7696cb7f9-jfbwq             1/1     Running             0              3m19s
    minio-0                             1/1     Running             0              3m19s
    minio-1                             1/1     Running             0              3m19s
    minio-2                             1/1     Running             0              3m19s
    minio-3                             1/1     Running             0              3m19s
    mongodb-85487bf568-qjw2k            1/1     Running             0              3m19s
    nack-644748c7b7-p6z62               1/1     Running             0              3m19s
    nats-0                              2/2     Running             0              3m19s
    nats-1                              2/2     Running             0              3m19s
    nats-2                              2/2     Running             0              3m19s
    terraformer-5868fb7695-w49sw        1/1     Running             0              3m19s
    ```

2. Check the `InputManifest` resource status to find out what is the actual cluster state.

    ```bash
    kubectl get inputmanifests.claudie.io <your-input-manifest-name> -o jsonpath={.status}
    ```

    ```text
      {
        "clusters": {
          "one-of-my-cluster": {
            "message": " installing VPN",
            "phase": "ANSIBLER",
            "state": "IN_PROGRESS"
          }
        },
        "state": "IN_PROGRESS"
      }
    ```

3. Examine claudie-operator service logs. The claudie-operator service logs will provide insights into any issues during cluster bootstrap and identify the problematic service. If cluster creation fails despite all Claudie pods being scheduled, it may suggest lack of permissions for Claudie providers' credentials. In this case, operator logs will point to Terrafomer service, and Terraformer service logs will provide detailed error output.

    ```bash
    kubectl -n claudie logs -l app.kubernetes.io/name=claudie-operator
    ```

    ```text
    6:04AM INF Using log with the level "info" module=claudie-operator
    6:04AM INF Claudie-operator is ready to process input manifests module=claudie-operator
    6:04AM INF Claudie-operator is ready to watch input manifest statuses module=claudie-operator
    ```

    !!! note "Debug log level"
        Using debug log level will help here with identifying the issue closely. [This guide](https://docs.claudie.io/v0.4.0/getting-started/detailed-guide/#claudie-deployment) shows how you can set it up during step 5.

    !!! note "Claudie benefit!"
        The great thing about Claudie is that it utilizes open source tools to set up and configure infrastructure based on your preferences. As a result, the majority of errors can be easily found and resolved through online resources.

### Terraformer service not starting
Terraformer relies on MinIO datastore to be configured via jobs `make-bucket-job`. If the job fails to configure the datastore, or the datastore itself fails to start, Terraformer will also fail to start.

### Datastore initialization jobs
The `make-bucket-job` creates a bucket in the MinIO datastore. If this job encounter scheduling problems or experience slow autoscaling, it may fail to complete within the designated time frame. To handle this, we have set the `backoffLimit` to fail after approximately 42 minutes. If you encounter any issues with this job or believe the `backoffLimit` should be adjusted, please [create an issue](https://github.com/berops/claudie/issues).

## Networking issues
### Wireguard MTU
We use Wireguard for secure node-to-node connectivity. However, it requires setting the MTU value to match that of Wireguard. While the host system interface MTU value is adjusted accordingly, networking issues may arise for services hosted on Claudie managed Kubernetes clusters. For example, we observed that the GitHub actions runner docker container had to be configured with an MTU value of `1380` to avoid network errors during `docker build` process.

### Hetzner and OCI node pools
We're experiencing networking issues caused by the blacklisting of public IPs owned by Hetzner and OCI. This problem affects the Ansibler and Kube-eleven services, which fail when attempting to add GPG keys to access the Google repository for package downloads. Unfortunately, there's no straightforward solution to bypass this issue. The recommended approach is to allow the services to fail, remove failed cluster and attempt provisioning a new cluster with newly allocated IP addresses that are not blocked by Google.

## Resolving issues with Tofu state lock

~During normal operation, the content of this section should not be required. If you ended up here, it means there was likely a bug somewhere in Claudie. Please [open a bug report](https://github.com/berops/claudie/issues/new/choose) in that case and use the content of this section to troubleshoot your way out of it.

First of all you have to get into the directory in the `terraformer` pod, where all terraform files are located. In order to do that, follow these steps:

* `kubectl exec -it -n claudie <terraformer-pod-name> -- bash`
* `cd ./services/terraformer/clusters/<cluster-name>`

### Locked state

Once you are in the directory with all TF files, run the following command:

```bash
tofu force-unlock <state-lock-id>
```

The `<state-lock-id>` is generally shown in the error message.
