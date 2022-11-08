# Input manifest

## Manifest

Manifest is a definition of the user's infrastructure. It contains cloud provider specification, nodepool specification, Kubernetes and loadbalancer clusters. 

- `name`

  Name of the manifest. Must be unique across all manifests of the Claudie instance.

- `providers` [Providers](#providers)

  Defines all your cloud provider configuration that will be used while infrastructure provisioning.

- `nodepools` [Nodepools](#nodepools)

  Describes nodepools used for either kubernetes clusters or loadbalancer cluster defined in this manifest.

- `kubernetes` [Kubernetes](#kubernetes)

  List of Kubernetes cluster this manifest will manage.

- `loadBalancers` [Loadbalancer](#loadbalancer)

  List of loadbalancer clusters the Kubernetes clusters may use.

## Providers 

Contains configurations for different supported cloud providers. Atleast one provider needs to be defined out of the following supported providers 

- `gcp` [GCP](#gcp)
  
  List of GCP configurations for [Google cloud](https://cloud.google.com/). This field is optional.

- `hetzner` [Hetzner](#hetzner)
  
  List of Hetzner configuration for [Hetzner cloud](https://www.hetzner.com/cloud) . This field is optional.

- `oci` [OCI](#oci)
  
  List of OCI configuration for [Oracle cloud infrastructure](https://www.oracle.com/uk/cloud/) . This field is optional.

- `aws` [AWS](#aws)
  
  List of AWS configuration for [Amazon web services](https://aws.amazon.com/) . This field is optional.

- `azure` [Azure](#azure)
  
  List of Azure configuration for [Azure](https://azure.microsoft.com/en-gb/) . This field is optional.

Support for more cloud provider is planned and will be rolled out in future. 

## GCP

Collection of data defining GCP cloud provider configuration. 

- `name`

  Name of the provider. Used as a reference further in the input manifest. Should be unique for each provider spec across all the cloud providers.

- `credentials`

  Credentials for the provider. Stringified JSON service account key.

- `gcp_project`

  GCP project id of an already existing GCP project.

## Hetzner

Collection of data defining Hetzner cloud provider configuration. 

- `name`

  Name of the provider spec. Used as a reference further in the input manifest. Should be unique for each provider spec across all the cloud providers.

- `credentials`

  Credentials for the provider (API token).

## OCI

Collection of data defining OCI cloud provider configuration. 

- `name`

  Name of the provider spec. Used as a reference further in the input manifest. Should be unique for each provider spec across all the cloud providers.

- `private_key`

  [Private key](https://docs.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm#two) used to authenticate to the OCI.

- `key_fingerprint`

  Fingerprint to the supplied private key.

- `tenancy_ocid`
  
  OCID of the tenancy where `private_key` is added as an API key

- `user_ocid`
  
  OCID of the user in the supplied tenancy

- `compartment_ocid`

  OCID of the compartment where VMs/VCNs/... will be created

## AWS

Collection of data defining AWS cloud provider configuration. 

- `name`

  Name of the provider spec. Used as a reference further in the input manifest. Should be unique for each provider spec across all the cloud providers.

- `access_key`

  Access key ID for your AWS account.

- `secret_key`

  Secret key for the Access key specified above.

## Azure

Collection of data defining Azure cloud provider configuration. 

- `name`

  Name of the provider spec. Used as a reference further in the input manifest. Should be unique for each provider spec across all the cloud providers.

- `subscription_id`

  Subscription ID of your subscription in Azure.

- `tenant_id`
  
  Tenant ID of your tenancy in Azure.

- `client_id`

  Client ID of your client. The Claudie is design to use a service principal with appropriate permissions.

- `client_secret`
  
  Client secret generated for your client.

- `resource_group`

  Resource group name where the infra will be created.

## Nodepools

Collection of static and dynamic nodepool specification, to be referenced in the `kubernetes` or `loadBalancer` clusters.

- `dynamic` [Dynamic](#dynamic)

  List of dynamically to-be-created nodepools of not yet existing machines, used for Kubernetes or loadbalancer clusters. 
  
  These are only blueprints, and will only be created per reference in `kubernetes` or `loadBalancer` clusters. E.g. if the nodepool isn't used, it won't even be created. Or if the same nodepool is used in two different clusters, it will be created twice.
In OOP analogy, a dynamic nodepool would be a class that would get instantiated `N >= 0` times depending on which clusters reference it.

- `static` [WORK IN PROGRESS]

  List of static nodepools of already existing machines, not created by of Claudie, used for Kubernetes or loadbalancer clusters. Typically, these would be on-premises machines.


## Dynamic

Dynamic nodepools are defined for cloud provider machines that Claudie is expected to create.

- `name`

  Name of the nodepool. Each nodepool will have a random hash appended to the name, so the whole name will be of format `<name>-<hash>`.

- `provideSpec` [Provider spec](#provider-spec)

  Collection of provider data to be used while creating the nodepool.  

- `count`

  Number of the nodes in the nodepool.

- `server_type`
  
  Type of the machines in the nodepool.

- `image`

  OS image of the machine. Currently, Claudie only supports ubuntu images.

- `disk_size`

  Size of the disk on the nodes in the nodepool.

## Provider Spec

Provider spec is further specification build on top of the data from either [GCP](#gcp) or [Hetzner](#hetzner)

- `name`

  Name of the provider specified in either [GCP](#gcp) or [Hetzner](#hetzner)

- `region`

  Region of the nodepool. [NOTE: only used in GCP nodepools]

- `zone`

  Zone of the nodepool. Zone can be either GCP zone or Hetzner datacenter.
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

  | Value | Description                |
  | ----- | -------------------------- |
  | `tcp` | Role will use TCP protocol |
  | `udp` | Role will use UDP protocol |

- `port`

  Port of the incoming traffic on the loadbalancer.

- `target_port`

  Port where loadbalancer forwards the traffic.

- `target` 

  Defines a target group of nodes. Allowed values are:

  | Value             | Description                          |
  | ----------------- | ------------------------------------ |
  | `k8sAllNodes`     | All nodes in the cluster             |
  | `k8sControlNodes` | Only control/master nodes in cluster |
  | `k8sComputeNodes` | Only compute/worker nodes in cluster |

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

- `provider`

  Name of [provider](#providers) to be used for creating an A record entry in defined dns zone.

- `hostname`
  
  Custom hostname for your A record. If left empty, the hostname will be a random hash.
