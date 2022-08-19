# Config spec
Config is a datastructure which holds all of the data for a Claudie microservices. It is saved in the database and is passed from service to service.

## Config
Config holds data for a single manifest

  | Name | Type | Description |
  |------|------|-------------|
  | id | string | Config id |
  | name | string | Config name |
  | manifest | string | Client defined manifest|
  | desiredState | [Project](#project) | Desired state from the manifest |
  | currentState | [Project](#project) | Current state of the infra |
  | msChecksum | bytes | Manifest state checksum |
  | dsChecksum | bytes | Desired state checksum |
  | csChecksum | bytes | Current state checksum |
  | builderTTL | int32 | Builder time to live counter |
  | schedulerTTL | int32 | Scheduler time to live counter |
  | errorMessage | string | Error message from error encountered during execution |

## Project
Project represents desired state of the manifest and current state of the manifest

  | Name | Type | Description |
  |------|------|-------------|
  | Name | string | Name of the project |
  | Clusters | [][K8scluster](#k8scluster) | Slice of kubernetes clusters |
  | LoadBalancerClusters | [][LBcluster](#lbcluster) | Slice of loadbalancers |

## K8scluster
K8scluster represents single kubernetes cluster specified in manifest

  | Name | Type | Description |
  |------|------|-------------|
  | ClusterInfo | [ClusterInfo](#clusterinfo) | General info about the cluster |
  | Network | string | Network range for the VPN |
  | Kubernetes | string | Kubernetes version |

## LBcluster
LBcluster represents single loadbalancer cluster specified in manifest
  | Name | Type | Description |
  |------|------|-------------|
  | ClusterInfo | [ClusterInfo](#clusterinfo) | General info about the cluster |
  | Roles | [][Role](#role) | Loadbalancer role |
  | DNS | [Dns](#dns) | DNS information |
  | TargetedK8s | string | Kubernetes cluster name of cluster this loadbalancer is assigned to |

## ClusterInfo
ClusterInfo holds general information about the clusters

  | Name | Type | Description |
  |------|------|-------------|
  | Name | string | Name of the cluster |
  | Hash | string | Random hash of the cluster |
  | Public_key | string | Public ssh key for the nodes |
  | Private_key | string | Private ssh key for the nodes |
  | Nodepools | [][Nodepool](#nodepool) | Slice of node pools this cluster is made of |

## Role
Role represents a single loadbalancer role from the manifest

  | Name | Type | Description |
  |------|------|-------------|
  | Name | string | Name of the role |
  | Protocol | string | Protocol loadbalancer will use |
  | Port | int32 | Loadbalancer port |
  | TargetPort | int32 | Port that loadbalancer will forward to |
  | Target | [Target](#target) | Targeted nodes |
  | RoleType | [RoleType](#roletype) | Type of the role |

## DNS
DNS holds general information about the DNS records

  | Name | Type | Description |
  |------|------|-------------|
  | DnsZone | string | DNS zone for the DNS records |
  | Hostname | string | User specified hostname |
  | Provider | [Provider](#provider) | Provider for the DNS records |
  | Endpoint | string | The whole hostname of the DNS record |

## NodePool
NodePool represent a single nodepool from the manifest

  | Name | Type | Description |
  |------|------|-------------|
  | Name | string | Name of the node pool |
  | Region | string | Region of the nodes |
  | ServerType | string | Machine type of the nodes |
  | Image | string | OS image of the nodes |
  | DiskSize | int32 | Disk size of the nodes |
  | Zone | string | Zone for the nodes |
  | Count | int32 | Count of the nodes |
  | Nodes | [][Node](#node) | Slice of Nodes |
  | Provider | [Provider](#provider) | Provider of the nodepools |
  | IsControl | bool | Flag to differentiate between control and compute nodes |

## Node
Node represent a single node from the nodepool

  | Name | Type | Description |
  |------|------|-------------|
  | Name | string | Name of the node |
  | Private | string | Private IP of the node in the VPN |
  | Public | string | Public IP of the node |
  | NodeType | [NodeType](#nodetype) | Type of the node |
  
## Provider
Provider represent a single provider from manifest

  | Name | Type | Description |
  |------|------|-------------|
  | SpecName | string | Provider name |
  | Credentials | string | Credentials of the provider |
  | GcpProject | string | GCP project (only required when using GCP as DNS provider) |
  | CloudProviderName | string | Cloud provider name. e.g. `gcp` and `hetzner`

## NodeType
NodeType specifies type of the node

  | Value | Description |
  |-------|-------------|
  | Worker | Worker node |
  | Master | Master node |
  | ApiEndpoint | Master node which is also API endpoint |

## Target
Target specifies which nodes are target by loadbalancer

  | Value | Description |
  |-------|-------------|
  | K8sAllNodes | All nodes in cluster |
  | K8sControlPlane | Only Master nodes |
  | K8sComputePlane | Only Compute nodes |

## RoleType
RoleType specifies the type of the role
  | Value | Description |
  |-------|-------------|
  | ApiServer | API server loadbalancer |
  | Ingress | Ingress loadbalancer | 

## ClusterType
ClusterType specifies type of the cluster
  | Value | Description |
  |-------|-------------|
  | K8s | Kubernetes cluster |
  | LB | Loadbalancer cluster |