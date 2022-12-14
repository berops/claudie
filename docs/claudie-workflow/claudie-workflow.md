# Claudie
### Single platform for multiple clouds

![claudie schema](claudie-diagram.jpg)

### Microservices
- [Context-box](https://github.com/Berops/claudie/tree/master/services/context-box)
- [Scheduler](https://github.com/Berops/claudie/tree/master/services/scheduler)
- [Builder](https://github.com/Berops/claudie/tree/master/services/builder)
- [Terraformer](https://github.com/Berops/claudie/tree/master/services/terraformer)
- [Ansibler](https://github.com/Berops/claudie/tree/master/services/ansibler)
- [Kube-eleven](https://github.com/Berops/claudie/tree/master/services/kube-eleven)
- [Kuber](https://github.com/Berops/claudie/tree/master/services/kuber)
- [Frontend](https://github.com/Berops/claudie/tree/master/services/frontend)

### Data stores
- [MongoDB](https://github.com/Berops/claudie/tree/master/manifests/claudie/mongo)
- [Minio](https://github.com/Berops/claudie/tree/master/manifests/claudie/minio)

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
Context box is "control unit" for Claudie. It holds pending configs, which need to be processed, periodically checks for new/changed configs and receives new configs from `frontend`.

### API

```go
  //Save config parsed by Frontend
  rpc SaveConfigFrontEnd(SaveConfigRequest) returns (SaveConfigResponse);
  //Save config parsed by Scheduler
  rpc SaveConfigScheduler(SaveConfigRequest) returns (SaveConfigResponse);
  //Save config parsed by Builder
  rpc SaveConfigBuilder(SaveConfigRequest) returns (SaveConfigResponse);
  //Get single config from database
  rpc GetConfigFromDB(GetConfigFromDBRequest) returns (GetConfigFromDBResponse);
  // *(NEEDS DELETION)*
  rpc GetConfigByName(GetConfigByNameRequest) returns (GetConfigByNameResponse);
  //Get config from scheduler queue of pending configs
  rpc GetConfigScheduler(GetConfigRequest) returns (GetConfigResponse);
  //Get config from builder queue of pending configs
  rpc GetConfigBuilder(GetConfigRequest) returns (GetConfigResponse);
  //Get all configs from database
  rpc GetAllConfigs(GetAllConfigsRequest) returns (GetAllConfigsResponse);
  // Sets the manifest to null which forces the deletion of the infra,
  // defined by the manifest, next time when the config will be picked up.
  rpc DeleteConfig(DeleteConfigRequest) returns (DeleteConfigResponse);
  // Deletes config from database.
  rpc DeleteConfigFromDB(DeleteConfigRequest) returns (DeleteConfigResponse);
```
### Flow

- Receives config file from Frontend calculates `msChecksum` and saves it to the database
- Periodically push config where `msChecksum` != `dsChecksum` to the schedulerQueue
- Periodically push config where `dsChecksum` != `csChecksum` to the builderQueue
- Receives config with desiredState from Scheduler
- Checks if `dsChecksum` == `msChecksum`
    - `true` -> saves it to the database
    - `false` -> it ignores this config
- Receives config with currentState from Builder and saves it to the database
- Checks if `csChecksum` == `dsChecksum`
    - `true` -> saves it to the database
    - `false` -> it ignores this config

##### `msChecksum` - manifest checksum
##### `dsChecksum` - desired state checksum
##### `csChecksum` - current state checksum


## Scheduler
Scheduler brings the infrastructure to a desired the state based on the manifest taken from the config received from Context-box.

Scheduler also monitors the health of current infrastructure and manages any operations based on actual health state (e.g. replacement of broken node, etc. *[work in progress]*).

### API
```
This service is a gRPC client, thus it does not provide any API
```

### Flow
- Periodically pulls `config` from Context-Box's `schedulerQueue`
- Creates desiredState with `dsChecksum` from config
- Sends `config` file back to Context-box


## Builder
Builder aligns the current state of the infrastructure with the desired state. It calls methods on `terraformer`, `ansibler`, `kube-eleven` and `kuber` in order to manage the infrastructure. It follows that Builder also takes care of deleting nodes from a kubernetes cluster by finding differences between `desiredState` and `currentState`.

### API
```
This service is a gRPC client, thus it does not provide any API
```

### Flow
- Periodically pulls `config` from Context-Box's `builderQueue`
- Calls Terraformer, Ansibler, Kube-eleven and Kuber
- Creates `currentState`
- Sends updated config with `currentState` to Context-box


## Terraformer
Terraformer creates or destroys infra specified in the desired state via Terraform calls. 

### API
```go
  //Builds the infrastructure based on the provided desired state (includes addition/deletion of stuff)
  rpc BuildInfrastructure(BuildInfrastructureRequest) returns (BuildInfrastructureResponse);
  //Destroys the infrastructure completely
  rpc DestroyInfrastructure(DestroyInfrastructureRequest) returns (DestroyInfrastructureResponse);
```

### Flow
- Receives `config` from Builder
- Uses Terraform to create infrastructure from `desiredState`
- Updates `currentState` in `config`
- On infrastructure deletion request, destroys the infra based on the current state

## Ansibler
Ansibler uses Ansible to set up:
  - Wireguard VPN between the nodes
  - nginx loadbalancer
  - installs dependencies for nodes in kubernetes cluster

### API

```go
  //InstallNodeRequirements installs any requirements there are on all of the nodes
  rpc InstallNodeRequirements(InstallRequest) returns (InstallResponse);
  //InstallVPN installs VPN between nodes in the k8s cluster and lb clusters
  rpc InstallVPN(InstallRequest) returns (InstallResponse);
  //SetUpLoadbalancers sets up the loadbalancers, DNS and verifies their configuration
  rpc SetUpLoadbalancers(SetUpLBRequest) returns (SetUpLBResponse);
  //TeardownLoadBalancers correctly destroys the Load-Balancers attached to a k8s cluster
  //by correctly choosing the new ApiServer endpoint.
  rpc TeardownLoadBalancers(TeardownLBRequest) returns (TeardownLBResponse);
```

### Flow
- Receives `configToDelete` from Builder for `TeardownLoadBalancers()`
  - Finds the new ApiEndpoing among the control nodes of the k8s-cluster.
  - Setups up the new certs for the endpoint to be reachable
- Receives `config` from Builder for `InstallVPN()`
  - Sets up the ansible inventory, and installs the Wireguard full mesh VPN via playbook
  - Updates `currentState` in a `config`
- Receives `config` from Builder for `InstallNodeRequirements()`
  - Sets up the ansible inventory, and install any requirements nodes in the infra might need
  - Updates `currentState` in a `config`
- Receives `config` from Builder for `SetUpLoadbalancers()`
  - Sets up the ansible inventory, and installs nginx loadbalancers
  - Creates and verifies the DNS configuration for the loadbalancers


## Kube-eleven
Kube-eleven uses kubeOne to set up kubernetes clusters.
If the cluster has already been built, it assures the cluster is healthy and running smoothly.

### API
```go
  //BuildCluster will build the kubernetes clusters as specified in provided config
  rpc BuildCluster(BuildClusterRequest) returns (BuildClusterResponse);
```

### Flow
- Receives `config` from Builder
- Generates kubeOne manifest from `desiredState`
- Uses kubeOne to provision a kubernetes cluster
- Updates `currentState` in `config`

## Kuber
Kuber manipulates the cluster resources using `kubectl`.

### API
```go
  // StoreClusterMetatada creates secret which holds the private key and public IP addresses of the cluster supplied.
  rpc StoreClusterMetadata(StoreClusterMetadataRequest) returns (StoreClusterMetadataResponse);
  // StoreClusterMetatada deletes secret which holds the private key and public IP addresses of the cluster supplied.
  rpc DeleteClusterMetadata(DeleteClusterMetadataRequest) returns (DeleteClusterMetadataResponse);
  //SetUpStorage installs Longhorn into the cluster
  rpc SetUpStorage(SetUpStorageRequest) returns (SetUpStorageResponse); 
  //StoreKubeconfig will create a secret which holds kubeconfig of the Claudie created cluster
  rpc StoreKubeconfig(StoreKubeconfigRequest) returns (StoreKubeconfigResponse);
  //DeleteKubeconfig will remove a secret that holds kubeconfig of Claudie created cluster
  rpc DeleteKubeconfig(DeleteKubeconfigRequest) returns (DeleteKubeconfigResponse);
  //DeleteNodes will delete specified nodes from a specified k8s cluster
  rpc DeleteNodes(DeleteNodesRequest) returns (DeleteNodesResponse);
```

### Flow
- Receives `config` from Builder for `SetUpStorage()`
- Applies longhorn deployment
- Receives `config` from Builder for `StoreKubeconfig()`
- Creates a kubernetes secret which holds kubeconfig of the Claudie-created cluster
- On infra deletion, deletes the secret kubeconfig secret of the cluster being deleted

## Frontend
Frontend is a layer between the user and Claudie. 
New manifests are added as a secret into the kubernetes cluster where `k8s-sidecar` saves them into Frontend's file system
and notifies the Frontend service via a HTTP request that the new manifests are now available.

### API
```
This service is a gRPC client, thus it does not provide any API
```

### Flow
- User applies a new secret holding a manifest
- `k8s-sidecar` detects it and saves it to Frontend's file system
- `k8s-sidecar` notifies Frontend via a HTTP request that changes have been made
- Frontend detects the new manifest and saves it to the database
- On deletion of user created secrets, Frontend initiates a deletion process of the manifest
