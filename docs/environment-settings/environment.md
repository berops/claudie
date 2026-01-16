Some Claudie aspects can be configured via environment variables read from a ConfigMap in the namespace where Claudie is deployed.

These values let you to customize service endpoints, log levels, and concurrency limits. The default ConfigMap that is
deployed with Claudie can be found in the [Claudie GitHub repository](https://github.com/berops/claudie/blob/master/manifests/claudie/.env).

The full list of environment variables is as follows:

```
# Log level for each individual service, can be info|debug|warn|error
GOLANG_LOG=info

# Endpoint configuration for terraformer service
TERRAFORMER_HOSTNAME=terraformer
TERRAFORMER_PORT=50052

# Endpoint configuration for ansibler service
ANSIBLER_HOSTNAME=ansibler
ANSIBLER_PORT=50053

# Endpoint configuration for kube eleven service
KUBE_ELEVEN_HOSTNAME=kube-eleven
KUBE_ELEVEN_PORT=50054

# Endpoint configuration for manager service
MANAGER_HOSTNAME=manager
MANAGER_PORT=50055

# Endpoint configuration for kuber service
KUBER_HOSTNAME=kuber
KUBER_PORT=50057

# Endpoint configuration for claudie-operator service
OPERATOR_HOSTNAME=claudie-operator
OPERATOR_PORT=50058

# Endpoint configuration for MinIO deployment.
BUCKET_URL=http://minio:9000
BUCKET_NAME=claudie-tf-state-files
AWS_REGION=local

# Defines from which namespace InputManifests should be watched for by Claudie.
# Default is all namespaces
CLAUDIE_NAMESPACES="dev"

# Defines how many concurrent workers will ping nodes of the currently built cluster. Only applicable to Kuber and Manager services.
# Default is 20
PING_CONCURRENT_WORKERS=20

# Defines how many clusters ansibler will work on concurrently.
# Default is 8
ANSIBLER_CONCURRENT_CLUSTERS=8

# Defines with how many forks each ansible playbook will be spawned.
# Default is 32
ANSIBLER_FORKS=32

# Defines how many clusters kube-eleven will work on concurrently.
# Default is 7
KUBE_ELEVEN_CONCURRENT_CLUSTERS=7

# Defines how many nodes will be patched concurrently.
# Default is 30
KUBER_WORKERS=30

# Defines how many clusters terraformer will work on concurrently.
# Default is 7
TERRAFORMER_CONCURRENT_CLUSTERS=7

# Defines how many resources will be worked on in parallel by tofu.
# Default is 40
TERRAFORMER_TOFU_PARALLELISM=40
```

Changes to the ConfigMap are reflected after the respective services are restarted.
