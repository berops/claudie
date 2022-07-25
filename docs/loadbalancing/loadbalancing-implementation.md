# Claudie Loadbalancer solution

## Loadbalancer

Loadbalancers created by Claudie are using Nginx to loadbalance the traffic into the cluster nodes. 

### Role 

The Claudie uses concept or roles while configuring the loadbalancers in input manifest. Each role represents a loadbalancer configuration for a particular use. Roles are then assigned to the Loadbalancer cluster. Single loadbalancer cluster can have multiple roles assigned. Role config consists of

- `name` 
  - name of the role
- `protocol` 
  - protocol which the role uses
  - options:
    - `tcp`
    - `udp` 
- `port`
  - port on the loadbalancer with incoming traffic
  - options:
    - number from `[0-65536]` 
- `target_port` 
  - port on the target nodes where the loadbalancer forwards traffic to
  - options:
    - number from `[0-65536]`
- `target` 
  - target of the loadbalancer
  - options:
    - `k8sAllNodes` - all cluster nodes as target nodes
    - `k8sControlNodes` - only control nodes as target nodes
    - `k8sComputeNodes` - only compute nodes as target nodes

### Targeted kubernetes cluster

Loadbalancer is assigned to the kubernetes cluster with a field `targeted-k8s`. This field is using `name` of the kubernetes cluster as a value. Currently, single loadbalancer can be assigned to only single kubernetes cluster.

### DNS

The Claudie creates and manages DNS for the loadbalancer. If user adds loadbalancer into their infrastructure via Claudie, it will create a DNS A record with the public IP of the loadbalancer machines behind it. When the loadbalancer configuration changes in any way e.g. adds/removes a node, changes hostname, changes target; the DNS record is reconfigured seamlessly by Claudie. This lifts the burden of the DNS management from the user. DNS config consists of

- `dns_zone`
  - DNS zone for the DNS records 
- `project`
  - GCP project id
  - Claudie currently supports only GCP cloud DNS 
- `hostname`
  - hostname for the DNS records
  - if left empty, random hash will be generated 

### Nodepools

Loadbalancers are build from user defined nodepools in `pools` field, similar to how kubernetes clusters are defined. These nodepools allows user to change/scale the loadbalancers according to their needs without any fuss. See nodepool definition for more information.

# Example of loadbalancer definition

```yaml
loadBalancers:
  roles:
    - name: apiserver-lb
      protocol: tcp
      port: 6443
      target_port: 6443
      target: k8sControlPlane
  clusters:
    - name: lb-1
      roles:
        - apiserver-lb
      targeted-k8s: production-cluster # k8s cluster name
      pools:
        - nodepool-1
      dns:
      dns_zone: dns-zone
      project: gcp-project-id
      hostname: production  # www.production.<dns-zone>
```