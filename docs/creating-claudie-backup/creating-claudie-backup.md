# Creating Claudie Backup

In this section we'll explain where the state of Claudie is and
backing up the necessary components and restoring them on a completely
new cluster.

## Claudie state

Claudie stores its state in 3 different places. 

-   Input Manifests are stored in <b>Mongo</b>.
-   Terraform/OpenTofu state files are stored in **MinIO**. This same **MinIO** instance is utilized for the locking mechanism, leveraging [S3 native state locking](https://opentofu.org/blog/opentofu-1-10-0/) in OpenTofu.
-   In flight scheduled tasks are stored within NATS.

These are the only services that will have a PVC attached to it, the other are stateless.

## Backing up Claudie

!!! note "During the backup procedure it is advised to scale down the following Claudie deployments (kuber, kube-eleven, ansibler, terraformer, manager) to 0 replicas to avoid issues"

### Using Velero

This is the primary backup and restore method.

!!! warning "Velero does not support HostPath volumes. If the PVCs in your management cluster are attached to such volumes (e.g. when running on Kind or MiniKube), the backup will not work. In this case, use the below manual backup method."

All resources that are deployed or created by Claudie can be identified with the following label:

```
    app.kubernetes.io/part-of: claudie
```

!!! note "If you want to include your deployed Input Manifests to be part of the backup you'll have to add the same label to them."

We'll walk through the following scenario step-by-step to back up claudie and then restore it. 

Claudie is already deployed on an existing Management Cluster and at least 1 Input Manifest has been applied. The state
is backed up and the Management Cluster is replaced by a new one on which we restore the state.

!!! note "To back up the resources we'll be using Velero version v1.18.0."

The following steps will all be executed with the existing Management Cluster in context.

1. To create a backup, Velero needs to store the state to external storage. The list of supported
   providers for the external storage can be found in the [link](https://velero.io/docs/v1.11/supported-providers/).
   In this guide we'll be using AWS S3 object storage for our backup.

   
2. Prepare the S3 bucket by following the first two steps in this [setup guide](https://github.com/vmware-tanzu/velero-plugin-for-aws#setup), excluding the installation step, as this will be different for our use-case.


!!! note "If you do not have the `aws` CLI locally installed, follow the [user guide](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-welcome.html) to set it up."

3. Execute the following command to install Velero on the Management Cluster.
```bash 
velero install \
--provider aws \
--plugins velero/velero-plugin-for-aws:v1.14.0 \
--bucket $BUCKET \
--secret-file ./credentials-velero \
--backup-location-config region=$REGION \
--snapshot-location-config region=$REGION \
--use-node-agent \
--default-volumes-to-fs-backup
```

Following the instructions in step 2, you should have a `credentials-velero` file with the access and secret keys for the aws setup. The env variables `$BUCKET` and `$REGION` should be set to the name and region for the bucket created in AWS S3.

By default Velero will use your default config `$HOME/.kube/config`, if this is not the config that points to your Management Cluster, you can override it with the `--kubeconfig` argument.

4. Backup claudie by executing
```bash
velero backup create claudie-backup --selector app.kubernetes.io/part-of=claudie
```

To track the progress of the backup execute
```bash
velero backup describe claudie-backup --details
```

From this point the new Management Cluster for Claudie is in context.
We expect that your default `kubeconfig` points to the new Management Cluster, if it does not, you can override it in the following commands using `--kubeconfig ./path-to-config`.

1. Repeat the step to install Velero, but now on the new Management Cluster.
2. Install cert manager to the new Management Cluster by executing:
```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.19.3/cert-manager.yaml
```
3. To restore the state that was stored in the S3 bucket execute
```bash
velero restore create --from-backup claudie-backup
```

Once all resources are restored, you should be able to deploy new input manifests and also modify existing infrastructure  without any problems.

### Manual backup

!!! note "During the backup procedure it is advised to scale down the following Claudie deployments (kuber, kube-eleven, ansibler, terraformer, manager) to 0 replicas to avoid issues"

Claudie is already deployed on an existing Management Cluster and at least 1 Input Manifest has been applied.

Create a directory where the backup of the state will be stored.

```bash
mkdir claudie-backup
```

Put your Claudie inputmanifests into the created folder, e.g. `kubectl get InputManifest -A -oyaml > ./claudie-backup/all.yaml`

We will now back up the state of the respective input manifests from MongoDB and MinIO and any in flight scheduled tasks from NATS.

```bash
kubectl get pods -n claudie

NAME                                READY   STATUS              RESTARTS       AGE
make-bucket-job-4mxw7               0/1     Completed           0              3m19s
minio-0                             1/1     Running             0              3m19s
minio-1                             1/1     Running             0              3m19s
minio-2                             1/1     Running             0              3m19s
minio-3                             1/1     Running             0              3m19s
mongodb-85487bf568-qjw2k            1/1     Running             0              3m19s
nack-644748c7b7-p6z62               1/1     Running             0              3m19s
nats-0                              2/2     Running             0              3m19s
nats-1                              2/2     Running             0              3m19s
nats-2                              2/2     Running             0              3m19s
```

To backup state from MongoDB execute the following command

```bash
kubectl exec -n claudie <mongodb-pod-name> -- sh -c 'mongoexport --uri=mongodb://$MONGO_INITDB_ROOT_USERNAME:$MONGO_INITDB_ROOT_PASSWORD@localhost:27017/claudie -c inputManifests --authenticationDatabase admin' > claudie-backup/inputManifests
```

Next we need to backup the state from MinIO. Port-forward the MinIO service so that it is accessible from localhost.

```bash
kubectl port-forward -n claudie svc/minio 9000:9000
```

Setup an alias for the [mc](https://min.io/docs/minio/linux/reference/minio-mc.html) command line tool.

```bash
mc alias set claudie-minio http://127.0.0.1:9000 <your-access-key> <your-secret-key>
```

!!! note "Provide the access and secret key for minio. The default can be found in the github repository in the `manifests/claudie/minio/secrets` folder. If you have not changed them, we strongly encourage you to do so!"

Download the state into the backup folder

```bash
mc mirror claudie-minio/claudie-tf-state-files ./claudie-backup/<minio-backup-folder>
```

Finally, to backup NATS

```bash
kubectl port-forward svc/nats -n claudie 4222:4222
```

```bash
natscli account backup claudie-backup/<nats-backup-folder>
```

You now have everything you need to restore your input manifests to a new management cluster.

!!! warning "These files will contain your credentials, DO NOT STORE THEM OUT IN THE PUBLIC!"

To restore the state on your new management cluster you can follow these commands. We expect that your default `kubeconfig` points to the new Management Cluster, if it does not, you can override it in the following commands using `--kubeconfig ./path-to-config`. Also that the Claudie services are scaled down to 0 replicas, to avoid any issues.

Copy the collection into the MongoDB pod.

```bash
kubectl cp ./claudie-backup/inputManifests mongodb-<mongodb-pod-name>:/tmp/inputManifests -n claudie
```

Import the state to MongoDB.

```bash
kubectl exec -n claudie mongodb-<mongodb-pod-name> -- sh -c 'mongoimport --uri=mongodb://$MONGO_INITDB_ROOT_USERNAME:$MONGO_INITDB_ROOT_PASSWORD@localhost:27017/claudie -c inputManifests --authenticationDatabase admin --file /tmp/inputManifests'
```

!!! note "Don't forget to delete the `/tmp/inputManifests` file"

Port-forward the MinIO service and import the backed up state.

```bash
mc cp --recursive ./claudie-backup/<minio-backup-folder> claudie-minio/claudie-tf-state-files
```

Port-forward the NATS service, first delete `claudie-internal` stream, if it exists.

```bash
natscli stream rm claudie-internal
```

Then you will be able to restore the backup.

```bash
natscli account restore claudie-backup/<nats-backup-folder>
```

You can now scale the deployments back and apply your Claudie inputmanifests and any state that was restored should continue operating as normal.

```bash
kubectl get inputmanifests -A
```

Now you can make any new changes to your inputmanifests on the new management cluster and the state will be re-used. 

!!! note "The secrets for the clusters, namely kubeconfig and cluster-metadata, are re-created after the workflow with the changes has finished."

!!! note "Alternatively you may also use any GUI clients for MongoDB and Minio for more straightforward backup of the state. All you need to backup is the bucket `claudie-tf-state-files` in MinIO and the collection `inputManifests` from MongoDB"

Once all data is restored, you should be able to deploy new input manifests and also modify existing infrastructure  without any problems.
