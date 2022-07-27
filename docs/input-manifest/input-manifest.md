# Input manifest

## Manifest

Manifest is a definition of the user's infrastructure. It contains cloud provider specification, nodepool specification, Kubernetes and loadbalancer clusters. 

- `name`

  Name of the manifest. Must be unique across all manifests of the Claudie instance.

- `providers` [Provider](#providers)

  List of Cloud providers used for the infrastructure. Includes DNS provider, Nodepool provider, etc.

- `nodepools` [Nodepools](#nodepools)

  Describes nodepools used for either kubernetes clusters or loadbalancer cluster defined in this manifest.

- `kubernetes` [Kubernetes](#kubernetes)

  List of Kubernetes cluster this manifest will manage.

- `loadBalancers` [Loadbalancer](#loadbalancer)

  List of loadbalancer clusters the Kubernetes clusters may use.

## Provider [NEEDS REWORK]

Collection of data defining a used cloud provider, like Hetzner or GCP.

- `name`

  Name of the provider. Used as a reference further in the input manifest.

- `credentials`

  Credentials for the provider. Either an API token (Hetzner), or a service account key (GCP).

- `gcp_project`

  GCP project id of an already existing GCP project. Only valid for GCP.

## Nodepools

Collection of static and dynamic nodepool specification. These are "blueprints" for the nodepools, which will be created once referenced in the `kubernetes` or `loadBalancer` clusters. This allows you to use the same nodepool for multiple purposes.

- `dynamic` [Dynamic](#dynamic)

  Collection of dynamically created nodepools of not yet existing machines, used for Kubernetes or loadbalancer clusters.

- `static` [WORK IN PROGRESS]

  Collection of statically created nodepools of already existing machines, not created by of Claudie, used for Kubernetes or loadbalancer clusters. Typically, these would be on-premises machines.


## Dynamic

Dynamic nodepools are defined for cloud provider machines that Claudie is expected to create.

- `name`

  Name of the nodepool. Each nodepool will have a random hash appended to the name, so the whole name will be of format `<name>-<hash>`.

- `provider`

  Provider of the nodepool [NEEDS REWORK]

- `count`

  Number of the nodes in the nodepool.

- `server_type`
  
  Type of the machines in the nodepool.

- `image`

  OS image of the machine. Currently, Claudie only supports ubuntu images.

- `disk_size`

  Size of the disk on the nodes in the nodepool.


## Kubernetes

Defines Kubernetes clusters.

- `clusters` [Cluster-k8s](#cluster-k8s)

  List of Kubernetes clusters Claudie will create.

## Cluster-k8s

Collection of data used to define a Kubernetes cluster.

- `name`

  Name of the Kubernetes cluster. Each cluster will have a random hash appended to the name, so the whole name will be of format `<name>-<hash>`.

- `version`

  Kubernetes version of the cluster. The version should be defined in format `vX.Y`.

- `network`

  Network range for the VPN of the cluster. The value should be defined in format `A.B.C.D/mask`.

- `pools`

  List of nodepool names this cluster will use. Remember that nodepools defined in [nodepools](#nodepools) are only "blueprints". The actual nodepool will be created once referenced here. 

## LoadBalancer

Defines loadbalancer clusters.

- `roles` [Role](#role)
  
  List of roles loadbalancers use to forward the traffic. Single role can be used in multiple loadbalancer clusters.

- `clusters` [Cluster-lb](#cluster-lb)

  List of loadbalancer clusters used in the Kubernetes clusters defined under [clusters](#cluster-k8s).

## Role

Role defines a concrete loadbalancer configuration. Single loadbalancer can have multiple roles.

- `name`

  Name of the role. Used as a reference in [clusters](#cluster-lb).

- `protocol`

  Protocol of the rule. Allowed values are:

  | Value | Description |
  |-------|-------------|
  | `tcp` | Role will use TCP protocol |
  | `udp` | Role will use UDP protocol |

- `port`

  Port of the incoming traffic on the loadbalancer.

- `target_port`

  Port where loadbalancer forwards the traffic.

- `target` 

  Defines a target group of nodes. Allowed values are:

  | Value | Description |
  |-------|-------------|
  |`k8sAllNodes` | All nodes in the cluster |
  |`k8sControlNodes` | Only control/master nodes in cluster |
  |`k8sComputeNodes` | Only compute/worker nodes in cluster |

## Cluster-lb

Collection of data used to define a loadbalancer cluster.

- `name`

  Name of the loadbalancer.

- `roles`
  
  List of roles the loadbalancer uses.

- `dns` [DNS](#dns)
  
  Specification of the loadbalancer's DNS record.
  
- `targeted-k8s`

  Name of the Kubernetes cluster targetted by this loadbalancer.

- `pools`

  List of nodepool names this loadbalancer will use. Remember, that nodepools defined in [nodepools](#nodepools) are only "blueprints". The actual nodepool will be created once referenced here. 

## DNS

Collection of data Claudie uses to create a DNS record for the loadbalancer.

- `dns_zone`

  DNS zone inside of which the records will be created. For now, only a GCP DNS zone is accepted, thus making definition of the GCP provider necessary.

- `project`

  [NEEDS REWORK]  

- `hostname`
  
  Custom hostname for your record. If left empty, the hostname will be a random hash.
