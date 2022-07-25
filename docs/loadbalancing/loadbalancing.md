# Claudie Loadbalancer solution

## Requirements

- 1 or more virtual machines (LB machines), either on-premises, or via 1 on more cloud providers.
- All the LB machines must have public IP addresses
    - Cloud provider LB machines are provisioned by Claudie with their own public IP address
    - On-premises LB machines are already expected to exist with their own public IP address

## Concept

- The machines will join the Wireguard private network of Claudie clusters relevant to it
  - This is necessary so that the LB machines can send traffic to the cluster machines over the `wireguard VPN`
  
- DNS A records will be created and managed by Claudie on 1 or more cloud providers
  - There will be a DNS A record for the public IP of each LB machine that is currently passing the health checks

- The LB machines will run an `Nginx` to carry out the actual load balancing.
  - There will be a DNS A record for the public IP of each LB machine that is currently passing the health checks
  - Therefore, there will be actually 2 layers of load balancing 
    1. DNS-based load balancing to determine the LB machine to be used 
    2. Software load balancing on the chosen LB machine. 

- Claudie will dynamically manage the LB configuration, e.g. if some cluster node is removed, the LB configuration changes or DNS configuration changes (hostname change)

- The load balancing will be on L4 layer, TCP/UDP, partially configurable by the Claudie input manifest
  - Example of input fields
```yaml
loadBalancers:
  roles:
    - name: apiserver-lb
      protocol: tcp
      port: 6443
      target_port: 6443
      target: k8sControlPlane
  clusters:
    - name: loadbalancer-1
    roles:
        - apiserver-lb
      dns:
        dns_zone: dns-zone
        project: project-id
        hostname: claudie-client
      targeted-k8s: k8s-cluster-1 # k8s cluster name
      pools:
        - lb-nodepool
```
- Note that a pure `on-premises` DNS managed via Claudie is not going to be supported
  - But if the client wants, they can manually create a DNS record for their LB VM public IP addresses, outside Claudie