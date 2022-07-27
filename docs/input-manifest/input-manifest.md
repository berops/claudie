# Input manifest

## Manifest

Manifest is a definition of a clients infrastructure. It contains cloud provider specification, nodepool specification, kubernetes and loadbalancer clusters. 

- `name`

Name of the manifest. If you plan to use multiple manifests for your infra, it is up to you to assure uniqueness of the name.

- `providers` [[]Provider](#providers)

List of provider used for the infrastructure. Includes DNS provider, Nodepool provider, etc.

- `nodepools` [Nodepools](#nodepools)

Nodepools field describes nodepools used for either kubernetes clusters or loadbalancer cluster defined in this manifest

- `kubernetes` [Kubernetes](#kubernetes)

Kubernetes is a list of kubernetes cluster this manifest will manage.

- `loadBalancers` [Loadbalancer](#loadbalancer)

LoadBalancers is a list of loadbalancer clusters the kubernetes clusters defined in `kubernetes` will use.

## Provider [NEEDS REWORK]

Provider is a collection of data for a cloud provider like Hetzner of GCP.

- `name`

Name of the provider. This name will be used to reference it further in the input manifest.

- `credentials`

Credentials for the provider. Can be API token, or a service account key.

- `gcp_project`

GCP specific field. Value is a project id for that particular credentials.

## Nodepools

Nodepools is a collection of static and dynamic nodepool specification. These are a "blueprints" for the nodepools, which will be created once referenced in the `kubernetes` or `loadBalancer` clusters. This allows you to define a single nodepool but use it in multiple clusters.

- `dynamic` [[]Dynamic](#dynamic)

Dynamic is a collection of dynamically created nodepool used for kubernetes or loadbalancer clusters.

- `static` [WORK IN PROGRESS]

Static is a collection of statically created nodepool, outside of Claudie used for kubernetes or loadbalancer clusters. Usually these would involve on premise machines.


## Dynamic

Dynamic nodepools are defined for cloud provider machines.

- `name`

Name of a nodepool. Each nodepool will have a random hash appended to the name, so the whole name will look like `<name>-<hash>`

- `provider`

Provider of a nodepool [NEEDS REWORK]

- `count`

Count of the nodes in the nodepools.

- `server_type`
  
Type of a machines in the nodepools.

- `image`

OS image of the machine. Currently, Claudie only supports ubuntu images.

- `disk_size`

Size of the disk on the nodes in the nodepool.


## Kubernetes

Kubernetes field is used to define a kubernetes clusters. 

- `clusters` [[]Cluster-k8s](#cluster-k8s)

Clusters is a list of kubernetes cluster Claudie will create.

## Cluster-k8s

Cluster is collection of data used to define a kubernetes cluster.

- `name`

Name of the kubernetes cluster. Each cluster will have a random hash appended to the name, so the whole name will look like `<name>-<hash>`

- `version`

Version of a kubernetes the cluster will use. The version should be defined as `vX.X`

- `network`

Network range for the VPN. The value should be defined as `X.X.X.X/mask`

- `pools`

The list of nodepool names this cluster will use. Remember, that nodepools defined in [nodepools](#nodepools) are only "blueprints". The actual nodepool will be created once referenced here. 

## LoadBalancer

Loadbalancer field defines loadbalancer nodes and loadbalancer clusters.

- `roles` [[]Role](#role)
  
Roles is a list of roles loadbalancers use to forward the traffic. Single role can be used in multiple loadbalancer clusters.

- `clusters` [[]Cluster-lb](#cluster-lb)

Clusters is a list of loadbalancer clusters used in the kubernetes cluster defined under [clusters](#cluster-k8s)

## Role

Role defines a concrete loadbalancer configuration. Single loadbalancer can have multiple roles.

- `name`

Name of the role. It is used to reference it in [clusters](#cluster-lb)

- `protocol`

Protocol of the rule. Allowed values are 

  | Value | Description |
  |-------|-------------|
  | `tcp` | Role will use TCP protocol |
  | `udp` | Role will use UDP protocol |

- `port`

Port is a port of the incoming traffic on the loadbalancer.

- `target_port`

Target port is a port where loadbalancer forwards the traffic.

- `target` 
Target defines a target group of nodes. Allowed values are

  | Value | Description |
  |-------|-------------|
  |`k8sAllNodes` | All nodes in the cluster |
  |`k8sControlNodes` | Only control/master nodes in cluster |
  |`k8sComputeNodes` | Only compute/worker nodes in cluster |

## Cluster-lb

Cluster is collection of data used to define a loadbalancer cluster.

- `name`

Name of the loadbalancer.

- `roles`
  
A list of roles this loadbalancer uses.

- `dns` [DNS](#dns)
  
DNS specification used for creation of loadbalancer DNS record
- `targeted-k8s`

Name of a kubernetes cluster which this loadbalancer targets

- `pools`

The list of nodepool names this loadbalancer will use. Remember, that nodepools defined in [nodepools](#nodepools) are only "blueprints". The actual nodepool will be created once referenced here. 

## DNS

DNS is a collection of data Claudie uses to create DNS record for loadbalancer machines.

- `dns_zone` 

DNS zone where records will be created 

- `project` 

[NEEDS REWORK]  

- `hostname`
  
Custom hostname for your record. If left empty, the hostname will be random hash