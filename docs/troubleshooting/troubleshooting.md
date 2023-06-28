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
    NAME                           READY   STATUS      RESTARTS        AGE
    ansibler-5c6c776b75-82c2q      1/1     Running     0               8m10s
    builder-59f9d44596-n2qzm       1/1     Running     0               8m10s
    context-box-5d76c89b4d-tb6h4   1/1     Running     1 (6m37s ago)   8m10s
    create-table-job-jvs9n         0/1     Completed   1               8m10s
    dynamodb-68777f9787-8wjhs      1/1     Running     0               8m10s
    frontend-5755b7bc69-5l84h      1/1     Running     0               8m10s
    kube-eleven-64468cd5bd-qp4d4   1/1     Running     0               8m10s
    kuber-698c4564c-dhsvg          1/1     Running     0               8m10s
    make-bucket-job-fb5sp          0/1     Completed   0               8m10s
    minio-0                        1/1     Running     0               8m10s
    minio-1                        1/1     Running     0               8m10s
    minio-2                        1/1     Running     0               8m10s
    minio-3                        1/1     Running     0               8m10s
    mongodb-67bf769957-9ct5z       1/1     Running     0               8m10s
    scheduler-654cbd4b97-qwtbf     1/1     Running     0               8m10s
    terraformer-fd664b7ff-dd2h7    1/1     Running     0               8m9s
    ```

1. After verifying if any pods in a failed state, you can check the logs of the frontend service for each service. Typically, if all Claudie pods are scheduled but cluster creation fails, it may be due to missing permissions on the credentials used for Claudie providers.

    ```bash
    kubectl -n claudie logs -l app.kubernetes.io/name=frontend
    ```

    ```text
    6:04AM INF Using log with the level "info" module=frontend
    6:04AM INF Frontend is ready to process input manifests module=frontend
    6:04AM INF Frontend is ready to watch input manifest statuses module=frontend
    ...
    9:19AM WRN Retrying command terraform apply --auto-approve... (5/10) module=terraformer
    9:20AM WRN Error encountered while executing terraform apply --auto-approve : exit status 1 module=terraformer
    9:20AM INF Next retry in 300s... module=terraformer
    9:25AM WRN Retrying command terraform apply --auto-approve... (6/10) module=terraformer
    ```

    !!! note "Debug log level"
        Using debug log level will help here with identifying the issue closely. [This guide](https://docs.claudie.io/v0.4.0/getting-started/detailed-guide/#claudie-deployment) shows how you can set it up during step 5.

    !!! node "Claudie benefit!"
        The great thing about Claudie is that it utilizes open source tools to set up and configure infrastructure based on your preferences. As a result, the majority of errors can be easily found and resolved through online resources.

### Terraformer service not starting
Terraformer relies on minio and dynamodb services for proper startup. If these services fail to start, Terraformer will also fail to start.

### Create table job
Scheduling problems of dynamodb service or even very slow autoscaling, may cause `create-table-job` to fail to create necessary tables in dynamodb service in time. We have set the `backoffLimit` of the `create-table-job` to fail after approximately 42 minutes of trying. If you encounter any issues with the `create-table-job` or if you feel like the `backoffLimit` has to be extended, please [create an issue](https://github.com/berops/claudie/issues).

## Networking issues
### Wireguard MTU
We use Wireguard for secure node-to-node connectivity. However, it requires setting the MTU value to match that of Wireguard. While the host system interface MTU value is adjusted accordingly, networking issues may arise for services hosted on Claudie managed Kubernetes clusters. For example, we observed that the GitHub actions runner docker container had to be configured with an MTU value of `1380` to avoid network errors during `docker build` process.

### Hetzner and OCI node pools
We're experiencing networking issues caused by the blacklisting of public IPs owned by Hetzner and OCI. This problem affects the kuber service, which fails when attempting to add GPG keys to access the Google repository for package downloads. Unfortunately, there's no straightforward solution to bypass this issue. The recommended approach is to allow the kuber service to fail, remove failed cluster and attempt provisioning a new cluster with IP addresses that are not blocked by Google.
