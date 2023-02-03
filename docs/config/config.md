# Config spec

Config is a datastructure, which holds all of the data for Claudie microservices. It is saved in the database and passed from service to service.

## Config

Config holds data for a single manifest.

  | Name         | Type                | Description                                           |
  | ------------ | ------------------- | ----------------------------------------------------- |
  | id           | string              | Config id                                             |
  | name         | string              | Config name                                           |
  | manifest     | string              | Client defined manifest                               |
  | desiredState | [Project](#project) | Desired state from the manifest                       |
  | currentState | [Project](#project) | Current state of the infra                            |
  | msChecksum   | bytes               | Manifest state checksum                               |
  | dsChecksum   | bytes               | Desired state checksum                                |
  | csChecksum   | bytes               | Current state checksum                                |
  | builderTTL   | int32               | Builder time to live counter                          |
  | schedulerTTL | int32               | Scheduler time to live counter                        |
  | errorMessage | string              | Error message from error encountered during execution |

## Project

Project represents the desired and current state of the manifest.
  | Name                 | Type                         | Description                  |
  | -------------------- | ---------------------------- | ---------------------------- |
  | Name                 | string                       | Name of the project          |
  | Clusters             | [] [K8scluster](#k8scluster) | Slice of kubernetes clusters |
  | LoadBalancerClusters | [] [LBcluster](#lbcluster)   | Slice of load balancers      |

## K8scluster

K8scluster represents a single kubernetes cluster specified in the manifest.
  | Name        | Type                        | Description                    |
  | ----------- | --------------------------- | ------------------------------ |
  | ClusterInfo | [ClusterInfo](#clusterinfo) | General info about the cluster |
  | Network     | string                      | Network range for the VPN      |
  | Kubernetes  | string                      | Kubernetes version             |

## LBcluster

LBcluster represents a single load balancer cluster specified in the manifest.
  | Name        | Type                        | Description                                                          |
  | ----------- | --------------------------- | -------------------------------------------------------------------- |
  | ClusterInfo | [ClusterInfo](#clusterinfo) | General info about the cluster                                       |
  | Roles       | [] [Role](#role)            | Load balancer role                                                   |
  | DNS         | [Dns](#dns)                 | DNS information                                                      |
  | TargetedK8s | string                      | Kubernetes cluster name of cluster this load balancer is assigned to |

## ClusterInfo

ClusterInfo holds general information about the clusters.
  | Name        | Type                     | Description                                 |
  | ----------- | ------------------------ | ------------------------------------------- |
  | Name        | string                   | Name of the cluster                         |
  | Hash        | string                   | Random hash of the cluster                  |
  | Public_key  | string                   | Public ssh key for the nodes                |
  | Private_key | string                   | Private ssh key for the nodes               |
  | Nodepools   | [] [Nodepool](#nodepool) | Slice of node pools this cluster is made of |

## Role

Role represents a single loadbalancer role from the manifest.
  | Name       | Type                  | Description                             |
  | ---------- | --------------------- | --------------------------------------- |
  | Name       | string                | Name of the role                        |
  | Protocol   | string                | Protocol that load balancer will use    |
  | Port       | int32                 | Load balancer port                      |
  | TargetPort | int32                 | Port that load balancer will forward to |
  | Target     | [Target](#target)     | Targeted nodes                          |
  | RoleType   | [RoleType](#roletype) | Type of the role                        |

## DNS

DNS holds general information about the DNS records.
  | Name     | Type                  | Description                          |
  | -------- | --------------------- | ------------------------------------ |
  | DnsZone  | string                | DNS zone for the DNS records         |
  | Hostname | string                | User specified hostname              |
  | Provider | [Provider](#provider) | Provider for the DNS records         |
  | Endpoint | string                | The whole hostname of the DNS record |

## NodePool

NodePool represents a single nodepool from the manifest.
  | Name       | Type                  | Description                                             |
  | ---------- | --------------------- | ------------------------------------------------------- |
  | Name       | string                | Name of the node pool                                   |
  | Region     | string                | Region of the nodes                                     |
  | ServerType | string                | Machine type of the nodes                               |
  | Image      | string                | OS image of the nodes                                   |
  | DiskSize   | int32                 | Disk size of the nodes                                  |
  | Zone       | string                | Zone for the nodes                                      |
  | Count      | int32                 | Count of the nodes                                      |
  | Nodes      | [] [Node](#node)      | Slice of Nodes                                          |
  | Provider   | [Provider](#provider) | Provider of the nodepools                               |
  | IsControl  | bool                  | Flag to differentiate between control and compute nodes |

## Node

Node represents a single node from the nodepool.
  | Name     | Type                  | Description                       |
  | -------- | --------------------- | --------------------------------- |
  | Name     | string                | Name of the node                  |
  | Private  | string                | Private IP of the node in the VPN |
  | Public   | string                | Public IP of the node             |
  | NodeType | [NodeType](#nodetype) | Type of the node                  |
  
## Provider

Provider represents a single provider from the manifest.
  | Name                | Type   | Description                                                        |
  | ------------------- | ------ | ------------------------------------------------------------------ |
  | SpecName            | string | Provider name                                                      |
  | CloudProviderName   | string | Cloud provider name. e.g. `gcp`, `hetzner`, `oci`, etc.            |
  | Credentials         | string | [Secret Credentials](#secret-credentials) of the provider          |
  | GcpProject          | string | GCP project (only required when using GCP as DNS provider)         |
  | OciUserOcid         | string | OCID of the user                                                   |
  | OciTenancyOcid      | string | OCID of the tenancy                                                |
  | OciFingerprint      | string | Fingerprint of the private key saved in `Credentials`              |
  | OciCompartmentOcid  | string | OCID of the compartment                                            |
  | AwsAccessKey        | string | AWS access key to the secret key saved in the `Credentials`        |
  | AzureSubscriptionId | string | Azure ID of the subscription                                       |
  | AzureTenantId       | string | Azure ID of the Tenant                                             |
  | AzureClientId       | string | AzureID of the Client; the client secret is saved in `Credentials` |

### Secret credentials

The list of information saved in the `Credentials` field for each provider.
  | Provider    | Input Manifest field                                          |
  | ----------- | ------------------------------------------------------------- |
  | GCP         | [`credentials`](../input-manifest/input-manifest.md#gcp)      |
  | Hetzner     | [`credentials`](../input-manifest/input-manifest.md#hetzner)  |
  | AWS         | [`secret_key`](../input-manifest/input-manifest.md#aws)       |
  | OCI         | [`private_key`](../input-manifest/input-manifest.md#oci)      |
  | Azure       | [`client_secret`](../input-manifest/input-manifest.md#azure)  |
  | Hetzner DNS | [`api_token`](../input-manifest/input-manifest.md#hetznerdns) |
  | Cloudflare  | [`api_token`](../input-manifest/input-manifest.md#cloudflare) |

## NodeType

NodeType specifies the type of the node.
  | Value       | Description                                |
  | ----------- | ------------------------------------------ |
  | Worker      | Worker node                                |
  | Master      | Master node                                |
  | ApiEndpoint | Master node, which is also an API endpoint |

## Target

Target specifies which nodes are targeted by the load balancer.
  | Value           | Description          |
  | --------------- | -------------------- |
  | K8sAllNodes     | All nodes in cluster |
  | K8sControlPlane | Only Master nodes    |
  | K8sComputePlane | Only Compute nodes   |

## RoleType

RoleType specifies the type of the role.
  | Value     | Description              |
  | --------- | ------------------------ |
  | ApiServer | API server load balancer |
  | Ingress   | Ingress load balancer    |

## ClusterType

ClusterType specifies the type of the cluster.
  | Value | Description           |
  | ----- | --------------------- |
  | K8s   | Kubernetes cluster    |
  | LB    | Load balancer cluster |
