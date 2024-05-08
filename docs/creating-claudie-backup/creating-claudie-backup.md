# Creating Claudie Backup

In this section we'll explain where the state of Claudie is and
backing up the necessary components and restoring them on a completely
new cluster.

## Claudie state

Claudie stores its state in 3 different places. 

-   Input Manifests are stored in <b>Mongo</b>.
-   Terraform state files are stored in <b>MinIO</b>
-   Locking Mechanism for the state files is implemented via <b>DynamoDB</b>

These are the only services that will have a PVC attached to it, the other are stateless.

## Backing up Claudie

### Manual backup

Claudie is already deployed on an existing Management Cluster and at least 1 Input Manifest has been applied.

Create a directory where the backup of the state will be stored.

```bash
mkdir claudie-backup
```

Put your Claudie inputmanifests into the created folder, e.g. `kubectl get InputManifest -A -oyaml > ./claudie-backup/all.yaml`

We will now back up the state of the respective input manifests from MongoDB and MinIO.

```bash
kubectl get pods -n claudie

NAME                                READY   STATUS      RESTARTS      AGE
ansibler-6f4557cf74-b4dts           1/1     Running     0             18m
builder-5d68987c86-qdfd5            1/1     Running     0             18m
claudie-operator-6d9ddc7f8b-hv84c   1/1     Running     0             18m
context-box-5d75bfffc6-d9qfm        1/1     Running     0             18m
create-table-job-ghb9f              0/1     Completed   1             18m
dynamodb-6d65df988-c626j            1/1     Running     0             18m
kube-eleven-556cfdfd98-jq6hl        1/1     Running     0             18m
kuber-7f8cd4cd89-6ds2w              1/1     Running     0             18m
make-bucket-job-9mjft               0/1     Completed   0             18m
minio-0                             1/1     Running     0             18m
minio-1                             1/1     Running     0             18m
minio-2                             1/1     Running     0             18m
minio-3                             1/1     Running     0             18m
mongodb-6ccb5f5dff-ptdw2            1/1     Running     0             18m
scheduler-676bd4468f-6fjn8          1/1     Running     0             18m
terraformer-66c6f67d98-pwr9t        1/1     Running     0             18m
```

To backup state from MongoDB execute the following command

```bash
kubectl exec -n claudie mongodb-<your-mongdb-pod> -- sh -c 'mongoexport --uri=mongodb://$MONGO_INITDB_ROOT_USERNAME:$MONGO_INITDB_ROOT_PASSWORD@localhost:27017/claudie -c inputManifests --authenticationDatabase admin' > claudie-backup/inputManifests
```

Next we need to backup the state from MinIO. Port-forward the MinIO service so that it is accessible from localhost.

```bash
kubectl port-forward -n claudie svc/minio 9000:9000
```

Setup an alias for the [mc](https://min.io/docs/minio/linux/reference/minio-mc.html) command line tool.

```bash
mc alias set claudie-minio http://127.0.0.1:9000 <ACCESSKEY> <SECRETKEY>
```

!!! note "Provide the access and secret key for minio. The default can be found in the github repository in the `manifests/claudie/minio/secrets` folder. If you have not changed them, we strongly encourage you to do so!"

Download the state into the backup folder

```bash
mc mirror claudie-minio/claudie-tf-state-files ./claudie-backup
```

You now have everything you need to restore your input manifests to a new management cluster.

!!! warning "These files will contain your credentials, DO NOT STORE THEM OUT IN THE PUBLIC!"

To restore the state on your new management cluster you can follow these commands. We expect that your default `kubeconfig` points to the new Management Cluster, if it does not, you can override it in the following commands using `--kubeconfig ./path-to-config`.

Copy the collection into the MongoDB pod.

```bash
kubectl cp ./claudie-backup/inputManifests mongodb-<your-mongodb-pod>:/tmp/inputManifests -n claudie
```

Import the state to MongoDB.

```bash
kubectl exec -n claudie mongodb-<your-mongodb-pod> -- sh -c 'mongoimport --uri=mongodb://$MONGO_INITDB_ROOT_USERNAME:$MONGO_INITDB_ROOT_PASSWORD@localhost:27017/claudie -c inputManifests --authenticationDatabase admin --file /tmp/inputManifests'
```

!!! note "Don't forget to delete the `/tmp/inputManifests` file"

Port-forward the MinIO service and import the backed up state.

```bash
mc cp --recursive ./claudie-backup/<your-folder-name-downloaded-from-minio> claudie-minio/claudie-tf-state-files
```

You can now apply your Claudie inputmanifests which will be immediately in the `DONE` stage. You can verify this with

```bash
kubectl get inputmanifests -A
```

Now you can make any new changes to your inputmanifests on the new management cluster and the state will be re-used. 

!!! note "The secrets for the clusters, namely kubeconfig and cluster-metadata, are re-created after the workflow with the changes has finished."

!!! note "Alternatively you may also use any GUI clients for MongoDB and Minio for more straightforward backup of the state. All you need to backup is the bucket `claudie-tf-state-files` in MinIO and the collection `inputManifests` from MongoDB"


### Using Velero

!!! warning "Velero does not support HostPath volumes. If the PVCs in your management cluster are attached to such volumes, the backup will not work. In this case, use the above backup method."

All resources that are deployed or created by Claudie can be identified with the following label:

```
    app.kubernetes.io/part-of: claudie
```

!!! note "If you want to include your deployed Input Manifests to be part of the backup you'll have to add the same label to them."

We'll walk through the following scenario step-by-step to back up claudie and then restore it. 

Claudie is already deployed on an existing Management Cluster and at least 1 Input Manifest has been applied. The state
is backed up and the Management Cluster is replaced by a new one on which we restore the state.

!!! note "To back up the resources we'll be using Velero version v1.11.0"

The following steps will all be executed with the existing Management Cluster in context.

1. To create a backup, Velero needs to store the state to external storage. The list of supported
   providers for the external storage can be found in the [link](https://velero.io/docs/v1.11/supported-providers/).
   In this guide we'll be using AWS S3 object storage for our backup.

   
2. Prepare the S3 bucket by following the first two steps in this [setup guide](https://github.com/vmware-tanzu/velero-plugin-for-aws#setup), excluding the installation step, as this will be different for our use-case.


!!! note "If you do not have the aws CLI locally installed, follow the [user guide](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-welcome.html) to set it up."

3. Execute the following command to install Velero on the Management Cluster.
```bash 
velero install \
--provider aws \
--plugins velero/velero-plugin-for-aws:v1.6.0 \
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
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.12.0/cert-manager.yaml
```
3. To restore the state that was stored in the S3 bucket execute
```bash
velero restore create --from-backup claudie-backup
```

Once all resources are restored, you should be able to deploy new input manifests and also modify existing infrastructure  without any problems.
