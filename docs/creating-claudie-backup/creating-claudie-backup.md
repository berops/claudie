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

All resources that are deployed or created by Claudie can be identified with the following label:
[creating-claudie-backup.md](creating-claudie-backup.md)
```
    app.kubernetes.io/part-of: claudie
```[creating-claudie-backup.md](creating-claudie-backup.md)

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