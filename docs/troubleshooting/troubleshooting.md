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
    NAME                                   READY   STATUS      RESTARTS        AGE
    ansibler-5c6c776b75-82c2q              1/1     Running     0               8m10s
    builder-59f9d44596-n2qzm               1/1     Running     0               8m10s
    manager-5d76c89b4d-tb6h4               1/1     Running     1 (6m37s ago)   8m10s
    create-table-job-jvs9n                 0/1     Completed   1               8m10s
    dynamodb-68777f9787-8wjhs              1/1     Running     0               8m10s
    claudie-operator-5755b7bc69-5l84h      1/1     Running     0               8m10s
    kube-eleven-64468cd5bd-qp4d4           1/1     Running     0               8m10s
    kuber-698c4564c-dhsvg                  1/1     Running     0               8m10s
    make-bucket-job-fb5sp                  0/1     Completed   0               8m10s
    minio-0                                1/1     Running     0               8m10s
    minio-1                                1/1     Running     0               8m10s
    minio-2                                1/1     Running     0               8m10s
    minio-3                                1/1     Running     0               8m10s
    mongodb-67bf769957-9ct5z               1/1     Running     0               8m10s
    terraformer-fd664b7ff-dd2h7            1/1     Running     0               8m9s
    ```

2. Check the `InputManifest` resource status to find out what is the actual cluster state.

    ```bash
    kubectl get inputmanifests.claudie.io resourceName -o jsonpath={.status}
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
Terraformer relies on MinIO and DynamoDB datastores to be configured via jobs `make-bucket-job` and `create-table-job` respectively. If these jobs fail to configure the datastores, or the datastores themselves fail to start, Terraformer will also fail to start.

### Datastore initialization jobs
The `create-table-job` is responsible for creating necessary tables in the DynamoDB datastore, while the `make-bucket-job` creates a bucket in the MinIO datastore. If these jobs encounter scheduling problems or experience slow autoscaling, they may fail to complete within the designated time frame. To handle this, we have set the `backoffLimit` of both jobs to fail after approximately 42 minutes. If you encounter any issues with these jobs or believe the `backoffLimit` should be adjusted, please [create an issue](https://github.com/berops/claudie/issues).

## Networking issues
### Wireguard MTU
We use Wireguard for secure node-to-node connectivity. However, it requires setting the MTU value to match that of Wireguard. While the host system interface MTU value is adjusted accordingly, networking issues may arise for services hosted on Claudie managed Kubernetes clusters. For example, we observed that the GitHub actions runner docker container had to be configured with an MTU value of `1380` to avoid network errors during `docker build` process.

### Hetzner and OCI node pools
We're experiencing networking issues caused by the blacklisting of public IPs owned by Hetzner and OCI. This problem affects the Ansibler and Kube-eleven services, which fail when attempting to add GPG keys to access the Google repository for package downloads. Unfortunately, there's no straightforward solution to bypass this issue. The recommended approach is to allow the services to fail, remove failed cluster and attempt provisioning a new cluster with newly allocated IP addresses that are not blocked by Google.

## Resolving issues with Terraform state lock

~During normal operation, the content of this section should not be required. If you ended up here, it means there was likely a bug somewhere in Claudie. Please [open a bug report](https://github.com/berops/claudie/issues/new/choose) in that case and use the content of this section to troubleshoot your way out of it.

First of all you have to get into the directory in the `terraformer` pod, where all terraform files are located. In order to do that, follow these steps:

* `kubectl exec -it -n claudie <terraformer-pod> -- bash`
* `cd ./services/terraformer/server/clusters/<your-cluster>`

### Locked state

Once you are in the directory with all TF files, run the following command:

```
tofu force-unlock <lock-id>
```

The `lock-id` is generally shown in the error message.
