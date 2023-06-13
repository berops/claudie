# Deploying Claudie

Claudie is designed to be deployed into the cluster with a single command

```sh
kubectl apply -k . # Point it to the directory where Claudie manifests reside
```

This will create all resources necessary to run the Claudie in your management cluster.

## Customisation

All of the customisable settings can be found in `.env` file.

| Variable               | Default       | Type   | Description                                                  |
| ---------------------- | ------------- | ------ | ------------------------------------------------------------ |
| `GOLANG_LOG`           | `info`        | string | Log level for all services. Can be either `info` or `debug`. |
| `DATABASE_HOSTNAME`    | `mongodb`     | string | Database hostname used for Claudie configs.                  |
| `CONTEXT_BOX_HOSTNAME` | `context-box` | string | Context-box service hostname.                                |
| `TERRAFORMER_HOSTNAME` | `terraformer` | string | Terraformer service hostname.                                |
| `ANSIBLER_HOSTNAME`    | `ansibler`    | string | Ansibler service hostname.                                   |
| `KUBE_ELEVEN_HOSTNAME` | `kube-eleven` | string | Kube-eleven service hostname.                                |
| `KUBER_HOSTNAME`       | `kuber`       | string | Kuber service hostname.                                      |
| `MINIO_HOSTNAME`       | `minio`       | string | MinIO hostname used for state files.                         |
| `DYNAMO_HOSTNAME`      | `dynamo`      | string | DynamoDB hostname used for lock files.                       |
| `DYNAMO_TABLE_NAME`    | `claudie`     | string | Table name for DynamoDB lock files.                          |
| `AWS_REGION`           | `local`       | string | Region for DynamoDB lock files.                              |
| `DATABASE_PORT`        | 27017         | int    | Port of the database service.                                |
| `TERRAFORMER_PORT`     | 50052         | int    | Port of the Terraformer service.                             |
| `ANSIBLER_PORT`        | 50053         | int    | Port of the Ansibler service.                                |
| `KUBE_ELEVEN_PORT`     | 50054         | int    | Port of the Kube-eleven service.                             |
| `CONTEXT_BOX_PORT`     | 50055         | int    | Port of the Context-box service.                             |
| `KUBER_PORT`           | 50057         | int    | Port of the Kuber service.                                   |
| `MINIO_PORT`           | 9000          | int    | Port of the MinIO service.                                   |
| `DYNAMO_PORT`          | 8000          | int    | Port of the DynamoDB service.                                |
