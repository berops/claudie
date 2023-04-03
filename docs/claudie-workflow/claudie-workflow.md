# Claudie

## A single platform for multiple clouds

![claudie schema](claudie-diagram.jpg)

### Microservices

- [Context-box](https://github.com/berops/claudie/tree/master/services/context-box)
- [Scheduler](https://github.com/berops/claudie/tree/master/services/scheduler)
- [Builder](https://github.com/berops/claudie/tree/master/services/builder)
- [Terraformer](https://github.com/berops/claudie/tree/master/services/terraformer)
- [Ansibler](https://github.com/berops/claudie/tree/master/services/ansibler)
- [Kube-eleven](https://github.com/berops/claudie/tree/master/services/kube-eleven)
- [Kuber](https://github.com/berops/claudie/tree/master/services/kuber)
- [Frontend](https://github.com/berops/claudie/tree/master/services/frontend)

### Data stores

- [MongoDB](https://github.com/berops/claudie/tree/master/manifests/claudie/mongo)
- [Minio](https://github.com/berops/claudie/tree/master/manifests/claudie/minio)
- [DynamoDB](https://github.com/berops/claudie/tree/master/manifests/claudie/dynamo)

### Tools used

- [Terraform](https://github.com/hashicorp/terraform)
- [Ansible](https://github.com/ansible/ansible)
- [KubeOne](https://github.com/kubermatic/kubeone)
- [Longhorn](https://github.com/longhorn/longhorn)
- [Nginx](https://www.nginx.com/)
- [Calico](https://github.com/projectcalico/calico)
- [K8s-sidecar](https://github.com/kiwigrid/k8s-sidecar)
- [gRPC](https://grpc.io/)

## Context-box

Context box is Claudie's "control unit". It holds pending configs, which need to be processed, periodically checks for new/changed configs and receives new configs from `frontend`.

### API

```go
  // SaveConfigFrontEnd saves the config parsed by Frontend.
  rpc SaveConfigFrontEnd(SaveConfigRequest) returns (SaveConfigResponse);
  // SaveConfigScheduler saves the config parsed by Scheduler.
  rpc SaveConfigScheduler(SaveConfigRequest) returns (SaveConfigResponse);
  // SaveConfigBuilder saves the config parsed by Builder.
  rpc SaveConfigBuilder(SaveConfigRequest) returns (SaveConfigResponse);
  // GetConfigFromDB gets a single config from the database.
  rpc GetConfigFromDB(GetConfigFromDBRequest) returns (GetConfigFromDBResponse);
  // GetConfigScheduler gets a config from Scheduler's queue of pending configs.
  rpc GetConfigScheduler(GetConfigRequest) returns (GetConfigResponse);
  // GetConfigBuilder gets a config from Builder's queue of pending configs.
  rpc GetConfigBuilder(GetConfigRequest) returns (GetConfigResponse);
  // GetAllConfigs gets all configs from the database.
  rpc GetAllConfigs(GetAllConfigsRequest) returns (GetAllConfigsResponse);
  // DeleteConfig sets the manifest to null, effectively forcing the deletion of the infrastructure
  // defined by the manifest on the very next config (diff-) check.
  rpc DeleteConfig(DeleteConfigRequest) returns (DeleteConfigResponse);
  // DeleteConfigFromDB deletes the config from the database.
  rpc DeleteConfigFromDB(DeleteConfigRequest) returns (DeleteConfigResponse);
  // UpdateNodepool updates specific nodepool from the config. Used mainly for autoscaling.
  rpc UpdateNodepool(UpdateNodepoolRequest) returns (UpdateNodepoolResponse);
```

### Flow

- Receives a `config` from Frontend, calculates its `msChecksum` and saves it to the database
- Periodically checks for `config` changes and pushes the `config` to the `schedulerQueue` if `msChecksum` != `dsChecksum`
- Periodically checks for `config` changes and pushes the `config` to the `builderQueue` if `dsChecksum` != `csChecksum`
- Receives a `config` with the `desiredState` from Scheduler and saves it to the database
- Receives a `config` with the `currentState` from Builder and saves it to the database

### Variables used

| variable     | meaning                |
| ------------ | ---------------------- |
| `msChecksum` | manifest checksum      |
| `dsChecksum` | desired state checksum |
| `csChecksum` | current state checksum |

## Scheduler

Scheduler brings the infrastructure to a desired the state based on the manifest contained in the config that is received from Context-box.

Scheduler also monitors the health of current infrastructure and manages any operations based on actual health state (e.g. replacement of broken nodes, etc. *[work in progress]*).

### API

>This service is a gRPC client, thus it does not provide any API

### Flow

- Periodically pulls `config` from Context-Box's `schedulerQueue`
- Creates `desiredState` with `dsChecksum` based on the `config`
- Sends the `config` file back to Context-box

## Builder

Builder aligns the current state of the infrastructure with the desired state. It calls methods on `terraformer`, `ansibler`, `kube-eleven` and `kuber` in order to manage the infrastructure. It follows that Builder also takes care of deleting nodes from a kubernetes cluster by finding differences between `desiredState` and `currentState`.

### API

>This service is a gRPC client, thus it does not provide any API

### Flow

- Periodically polls Context-Box's `builderQueue` for changes in `config`, pulls it when changed
- Calls Terraformer, Ansibler, Kube-eleven and Kuber
- Creates `currentState`
- Sends updated `config` with the `currentState` to Context-box

## Terraformer

Terraformer creates or destroys infrastructure (specified in the desired state) via Terraform calls.

### API

```go
  // BuildInfrastructure builds the infrastructure based on the provided desired state (includes addition/deletion of *stuff*).
  rpc BuildInfrastructure(BuildInfrastructureRequest) returns (BuildInfrastructureResponse);
  // DestroyInfrastructure destroys the infrastructure completely.
  rpc DestroyInfrastructure(DestroyInfrastructureRequest) returns (DestroyInfrastructureResponse);
```

### Flow

- Receives a `config` from Builder
- Uses Terraform to create infrastructure based on the `desiredState`
- Updates the `currentState` in the `config`
- Upon receiving a deletion request, Terraformer destroys the infrastructure based on the current state

## Ansibler

Ansibler uses Ansible to:

- set up Wireguard VPN between the nodes
- set up nginx load balancer
- install dependencies for nodes in a kubernetes cluster

### API

```go
  // InstallNodeRequirements installs any requirements there are on all of the nodes.
  rpc InstallNodeRequirements(InstallRequest) returns (InstallResponse);
  // InstallVPN sets up a VPN between the nodes in the k8s cluster and LB clusters.
  rpc InstallVPN(InstallRequest) returns (InstallResponse);
  // SetUpLoadbalancers sets up the load balancers together with the DNS and verifies their configuration.
  rpc SetUpLoadbalancers(SetUpLBRequest) returns (SetUpLBResponse);
  // TeardownLoadBalancers correctly destroys the load balancers attached to a k8s
  // cluster by choosing a new ApiServer endpoint.
  rpc TeardownLoadBalancers(TeardownLBRequest) returns (TeardownLBResponse);
```

### Flow

- Receives a `configToDelete` from Builder for `TeardownLoadBalancers()`
  - Finds the new ApiEndpoint among the control nodes of the k8s-cluster.
  - Sets up new certs for the endpoint to be reachable
- Receives a `config` from Builder for `InstallVPN()`
  - Sets up ansible *inventory*, and installs the Wireguard full mesh VPN using a playbook
  - Updates the `currentState` in a `config`
- Receives a `config` from Builder for `InstallNodeRequirements()`
  - Sets up ansible *inventory*, and installs any prerequisites, as per individual nodes' requirements
  - Updates the `currentState` in a `config`
- Receives a `config` from Builder for `SetUpLoadbalancers()`
  - Sets up the ansible inventory, and installs nginx load balancers
  - Creates and verifies the DNS configuration for the load balancers

## Kube-eleven

Kube-eleven uses [KubeOne](https://github.com/kubermatic/kubeone) to set up kubernetes clusters.
After cluster creation, it assures the cluster stays healthy and keeps running smoothly.

### API

```go
  // BuildCluster builds the kubernetes clusters specified in the provided config.
  rpc BuildCluster(BuildClusterRequest) returns (BuildClusterResponse);
```

### Flow

- Receives a `config` object from Builder
- Generates KubeOne manifest based on the `desiredState`
- Uses KubeOne to provision a kubernetes cluster
- Updates the `currentState` in the `config`

## Kuber

Kuber manipulates the cluster resources using `kubectl`.

### API

```go
  // RemoveLbScrapeConfig removes scrape config for every LB detached from this cluster.
  rpc RemoveLbScrapeConfig(RemoveLbScrapeConfigRequest) returns (RemoveLbScrapeConfigResponse);
  // StoreLbScrapeConfig stores scrape config for every LB attached to this cluster.
  rpc StoreLbScrapeConfig(StoreLbScrapeConfigRequest) returns (StoreLbScrapeConfigResponse);
  // StoreClusterMetadata creates a secret, which holds the private key and a list of public IP addresses of the cluster supplied.
  rpc StoreClusterMetadata(StoreClusterMetadataRequest) returns (StoreClusterMetadataResponse);
  // DeleteClusterMetadata deletes the secret holding the private key and public IP addresses of the cluster supplied.
  rpc DeleteClusterMetadata(DeleteClusterMetadataRequest) returns (DeleteClusterMetadataResponse);
  // SetUpStorage installs Longhorn into the cluster.
  rpc SetUpStorage(SetUpStorageRequest) returns (SetUpStorageResponse); 
  // StoreKubeconfig creates a secret, which holds the kubeconfig of a Claudie-created cluster.
  rpc StoreKubeconfig(StoreKubeconfigRequest) returns (StoreKubeconfigResponse);
  // DeleteKubeconfig removes the secret that holds the kubeconfig of a Claudie-created cluster.
  rpc DeleteKubeconfig(DeleteKubeconfigRequest) returns (DeleteKubeconfigResponse);
  // DeleteNodes deletes the specified nodes from a k8s cluster.
  rpc DeleteNodes(DeleteNodesRequest) returns (DeleteNodesResponse);
  // PatchNodes uses kubectl patch to change the node manifest.
  rpc PatchNodes(PatchNodeTemplateRequest) returns (PatchNodeTemplateResponse);
  // SetUpClusterAutoscaler deploys Cluster Autoscaler and Autoscaler Adapter for every cluster specified.
  rpc SetUpClusterAutoscaler(SetUpClusterAutoscalerRequest) returns (SetUpClusterAutoscalerResponse);
  // DestroyClusterAutoscaler deletes Cluster Autoscaler and Autoscaler Adapter for every cluster specified.
  rpc DestroyClusterAutoscaler(DestroyClusterAutoscalerRequest) returns (DestroyClusterAutoscalerResponse);
```

### Flow

- Receives a `config` from Builder for `SetUpStorage()`
- Applies the `longhorn` deployment
- Receives a `config` from Builder for `StoreKubeconfig()`
- Creates a kubernetes secret that holds the kubeconfig of the Claudie-created cluster
- Receives a `config` from Builder for `StoreMetadata()`
- Creates a kubernetes secret that holds the node metadata of the Claudie-created cluster
- Receives a `config` from Builder for `StoreLbScrapeConfig()`
- Stores scrape config for any LB attached to the Claudie-made cluster.
- Receives a `config` from Builder for `PatchNodes()`
- Patches the node manifests of the Claudie-made cluster.
- Upon infrastructure deletion request, Kuber deletes the kubeconfig secret, metadata secret, scrape configs and autoscaler of the cluster being deleted

## Frontend

Frontend is a layer between the user and Claudie.
New manifests are added as secrets into the kubernetes cluster where `k8s-sidecar` saves them into Frontend's file system
and notifies the Frontend service via a HTTP request that the new manifests are now available.

### API

>This service is a gRPC client, thus it does not provide any API

### Flow

- User applies a new secret holding a manifest
- `k8s-sidecar` detects it and saves it to Frontend's file system
- `k8s-sidecar` notifies Frontend via a HTTP request that changes have been made
- Frontend detects the new manifest and saves it to the database
- Upon deletion of user-created secrets, Frontend initiates a deletion process of the manifest
