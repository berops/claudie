# Claudie load balancing solution

## Loadbalancer

To create a highly available kubernetes cluster, Claudie has the option to create load balancers that utilize [envoy](https://www.envoyproxy.io/docs/envoy/latest/) to load balance the traffic among the cluster nodes.

The DNS load balancing functionality, including health checks, is provided by supported cloud providers such as AWS, Azure, Google Cloud, Cloudflare, and OCI. Health checks monitor TCP port 65534. If a node fails to respond on this port, its corresponding DNS record is temporarily removed. Once the endpoint becomes healthy again, the DNS record is automatically restored.

## Concept

- The load balancer machines will join the Wireguard private network of Claudie clusters relevant to it.
    - This is necessary so that the LB machines can send traffic to the cluster machines over the `wireguard VPN`.

- DNS A records will be created and managed by Claudie on 1 or more cloud providers.
    - There will be a DNS A record for the public IP of each LB machine that is currently passing the health checks.

- The LB machines will deploy a docker container running [envoy](https://www.envoyproxy.io/docs/envoy/latest/) for each role the loadbalancer uses, to carry out the actual load balancing.
    - There will be a DNS A record for the public IP of each LB machine that is currently passing the health checks.
    - Therefore, there will be actually 2 layers of load balancing.
        1. DNS-based load balancing to determine the LB machine to be used.
        2. Software load balancing on the chosen LB machine.

- Claudie will dynamically manage the LB configuration, e.g. if some cluster node is removed, the LB configuration changes or DNS configuration changes (hostname change).

- The load balancing will be on L4 layer, TCP/UDP, partially configurable by the Claudie input manifest.

## Example diagram

![lb-architecture](lb-architecture.png)

## Definitions

### Role

Claudie uses the concept of roles while configuring the load balancers from the input manifest. Each role represents a loadbalancer configuration for a particular use. Roles are then assigned to the load balancer cluster. A single load balancer cluster can have multiple roles assigned.

### Targeted kubernetes cluster

Load balancer gets assigned to a kubernetes cluster with the field `targetedK8s`. This field is using the `name` of the kubernetes cluster as a value. Currently, a single load balancer can only be assigned to a single kubernetes cluster.

**Among multiple load balancers targeting the same kubernetes cluster only one of them can have the API server role (i.e. the role with target port 6443) attached to it.**

### DNS

Claudie creates and manages the DNS for the load balancer. If the user adds a load balancer into their infrastructure via Claudie, Claudie creates a DNS A record with the public IP of the load balancer machines behind it. When the load balancer configuration changes in any way, that is a node is added/removed, the hostname or the target changes, the DNS record is reconfigured by Claudie on the fly. This rids the user of the need to manage DNS.

### Nodepools

Loadbalancers are build from user defined nodepools in `pools` field, similar to how kubernetes clusters are defined. These nodepools allow the user to change/scale the load balancers according to their needs without any fuss. See the nodepool definition for more information.

## An example of load balancer definition

See an example load balancer definition in our reference [example input manifest](../input-manifest/example.md).

## Notes

### Cluster ingress controller
You still need to deploy your own ingress controller to use the load balancer.
It needs to be set up to use `nodeport` with the ports configured under `roles` in the load balancer definition.
