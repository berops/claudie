# Claudie Loadbalancer solution

## Loadbalancer

For creating a highly available kubernetes cluster, Loadbalancers for kubeAPI-server is created by Claudie are using Nginx to loadbalance the traffic into the cluster nodes. 

### Role 

The Claudie uses concept or roles while configuring the loadbalancers in input manifest. Each role represents a loadbalancer configuration for a particular use. Roles are then assigned to the Loadbalancer cluster. Single loadbalancer cluster can have multiple roles assigned.

### Targeted kubernetes cluster

Loadbalancer is assigned to the kubernetes cluster with a field `targeted-k8s`. This field is using `name` of the kubernetes cluster as a value. Currently, single loadbalancer can be assigned to only single kubernetes cluster.

### DNS

The Claudie creates and manages DNS for the loadbalancer. If user adds loadbalancer into their infrastructure via Claudie, it will create a DNS A record with the public IP of the loadbalancer machines behind it. When the loadbalancer configuration changes in any way e.g. adds/removes a node, changes hostname, changes target; the DNS record is reconfigured seamlessly by Claudie. This lifts the burden of the DNS management from the user. 

### Nodepools

Loadbalancers are build from user defined nodepools in `pools` field, similar to how kubernetes clusters are defined. These nodepools allows user to change/scale the loadbalancers according to their needs without any fuss. See nodepool definition for more information.

# Example of loadbalancer definition
Example of loadbalancer definition can be found in [example manifest](../input-manifest/example.yaml)
