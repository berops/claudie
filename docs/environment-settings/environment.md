# Environment variables

Claudie services are configured via environment variables. Most of them are wired through a ConfigMap named `env`,
which is generated from the [`.env` file](https://github.com/berops/claudie/blob/master/manifests/claudie/.env)
shipped with the Claudie manifests.

How the variables reach the services:

- Each Deployment references specific keys of the `env` ConfigMap explicitly (there is no `envFrom`), so adding
  a new key to the ConfigMap alone has no effect unless the Deployment also references it.
- Any variable that is not set falls back to the default value built into the service (listed below).
- `DATABASE_URL` (manager) and `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` (terraformer) are sourced from the
  `mongo-secret` and `minio-secret` Secrets, not from the ConfigMap.
- `NAMESPACE` is injected automatically from the pod metadata, and `NATS_CLUSTER_URL` is set directly in each
  Deployment to `nats://nats.$(NAMESPACE).svc.cluster.local`.
- To set a variable that is not wired into a Deployment (e.g. `CLAUDIE_NAMESPACES` or `PING_CONCURRENT_WORKERS`),
  add it to the respective Deployment.

Changes to the ConfigMap are reflected after the respective services are restarted.

## Common

Variables shared by all services. They are read by every service, though some only take effect in
a specific service, noted in the description.

| Variable | Default | Description |
|----------|---------|-------------|
| `GOLANG_LOG` | `info` | Log level, one of `disabled`, `panic`, `fatal`, `error`, `warn`, `info`, `debug`, `trace`. |
| `NATS_CLUSTER_URL` | `nats://127.0.0.1` | URL of the NATS cluster used for inter-service messaging. Set by each Deployment to `nats://nats.$(NAMESPACE).svc.cluster.local`. |
| `NATS_CLUSTER_PORT` | `4222` | Port appended to `NATS_CLUSTER_URL`. |
| `NATS_CLUSTER_SIZE` | `1` | Expected size of the NATS cluster when connecting. The shipped `.env` sets `3`. |
| `NATS_CLUSTER_JETSTREAM_NAME` | `claudie-internal` | Name of the JetStream work queue through which tasks are exchanged. |
| `MANAGER_HOSTNAME` | `localhost` | Hostname under which the manager gRPC API is reachable. Used by the manager's clients (claudie-operator, testing-framework) to connect. The shipped `.env` sets `manager`. |
| `MANAGER_PORT` | `50055` | Port of the manager gRPC API. The manager listens on it; clients use it together with `MANAGER_HOSTNAME` to connect. |
| `DATABASE_URL` | `mongodb://localhost:27017` | MongoDB connection string for storing Claudie configs. Sourced from the `mongo-secret` Secret. Only acted upon by the manager. |
| `BUCKET_URL` | none | Endpoint of the MinIO/S3 deployment storing the tofu state files. When empty, an external AWS S3 bucket is used based on `AWS_REGION` and `BUCKET_NAME`. The shipped `.env` sets `http://minio:9000`. Only acted upon by terraformer. |
| `BUCKET_NAME` | `claudie-tf-state-files` | Name of the bucket holding the tofu state files. Only acted upon by terraformer. |
| `AWS_REGION` | `local` | Region of the bucket. Only acted upon by terraformer. |
| `AWS_ACCESS_KEY_ID` | `fake` | Access key for the bucket. Sourced from the `minio-secret` Secret. Only acted upon by terraformer. |
| `AWS_SECRET_ACCESS_KEY` | `fake` | Secret key for the bucket. Sourced from the `minio-secret` Secret. Only acted upon by terraformer. |
| `PROMETHEUS_PORT` | `9090` | Port of the `/metrics` HTTP endpoint. |
| `NAMESPACE` | none | Namespace of the Claudie deployment, injected automatically from the pod metadata. When empty, the services assume a local (non-Kubernetes) deployment. |

## Manager

| Variable | Default | Description |
|----------|---------|-------------|
| `MANAGER_TICK_FOR_INFRA_REFRESH` | `100` | Number of idle reconciliation ticks before a full infrastructure refresh is scheduled (100 ticks ≈ 35 minutes). |
| `MANAGER_TIME_FOR_NODE_DELETION` | `10` | How many minutes a node may stay unreachable before Claudie deletes and replaces it. |
| `PING_CONCURRENT_WORKERS` | `10` | How many concurrent workers ssh ping the loadbalancer nodes during healthchecks. |

## Terraformer

| Variable | Default | Description |
|----------|---------|-------------|
| `TERRAFORMER_PORT` | `50052` | Port of the terraformer gRPC healthcheck server. |
| `TERRAFORMER_CONCURRENT_PROCESSES` | `8` | How many tofu processes terraformer runs concurrently. |
| `TERRAFORMER_TOFU_PARALLELISM` | `40` | How many resources tofu works on in parallel (`--parallelism`). |

## Ansibler

| Variable | Default | Description |
|----------|---------|-------------|
| `ANSIBLER_PORT` | `50053` | Port of the ansibler gRPC healthcheck server. |
| `ANSIBLER_CONCURRENT_CLUSTERS` | `8` | How many clusters ansibler works on concurrently. |
| `ANSIBLER_FORKS` | `32` | With how many forks each ansible playbook is spawned (`--forks`). |

## Kube-eleven

| Variable | Default | Description |
|----------|---------|-------------|
| `KUBE_ELEVEN_PORT` | `50054` | Port of the kube-eleven gRPC healthcheck server. |
| `KUBE_ELEVEN_CONCURRENT_CLUSTERS` | `7` | How many clusters kube-eleven works on concurrently. |

## Kuber

| Variable | Default | Description |
|----------|---------|-------------|
| `KUBER_PORT` | `50057` | Port of the kuber gRPC healthcheck server. |
| `KUBER_CONCURRENT_WORKERS` | `30` | How many nodes are patched concurrently. |
| `KUBER_CONCURRENT_CALLS` | `90` | How many kubectl processes kuber runs concurrently. |

## Claudie-operator

| Variable | Default | Description |
|----------|---------|-------------|
| `OPERATOR_PORT` | `50058` | Port of the claudie-operator gRPC server. |
| `CLAUDIE_NAMESPACES` | all namespaces | Comma-separated list of namespaces in which InputManifests are watched for. See [deploying Claudie in a custom namespace](../input-manifest/claudie-custom-ns.md). |
| `WEBHOOK_TLS_PORT` | `9443` | TLS port of the InputManifest validating webhook. |
| `WEBHOOK_CERT_DIR` | `./tls` | Directory with the webhook server key and certificate. The shipped manifest sets `/etc/webhook/certs/`. |
| `WEBHOOK_PATH` | `/validate-manifest` | HTTP path served by the validating webhook. |
| `CLAUDIE_TEMPLATES_REFERENCE_HTTPS_URL` | `github.com/berops/claudie-config` | Repository of the default `claudie-default-templates` TemplateGitReference created by the operator. |
| `CLAUDIE_TEMPLATES_REFERENCE_COMMIT` | `release` | Ref/commit of the default TemplateGitReference. |

## Advanced service internals

Each of the manager, terraformer, ansibler, kube-eleven and kuber services additionally reads the following
variables, where `<SERVICE>` is the service name (e.g. `KUBE_ELEVEN_ACK_WAIT_TIME`). These rarely need to be changed.

| Variable | Default | Description |
|----------|---------|-------------|
| `<SERVICE>_DURABLE_NAME` | service name | Name of the durable NATS JetStream consumer. |
| `<SERVICE>_ACK_WAIT_TIME` | `8` | How many minutes an in-flight task message may stay unacknowledged before NATS redelivers it. |
| `<SERVICE>_HEALTHCHECK_READINESS_SERVICE_NAME` | `<service>-readiness` | gRPC health service name used by the readiness probe. |
| `<SERVICE>_HEALTHCHECK_LIVENESS_SERVICE_NAME` | `<service>-liveness` | gRPC health service name used by the liveness probe. |
