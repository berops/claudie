# Frequently Asked Question

We have prepared some of our most frequently asked question to help you out!

### Does Claudie make sense as a pure K8s orchestration on a single cloud-provider IaaS?

Since Claudie specializes in multicloud, you will likely face some drawbacks, such as the need for a public IPv4 address for each node. Otherwise it works well in a single-provider mode.
Using Claudie will also give you some advantages, such as scaling to multi-cloud as your needs change, or the autoscaler that Claudie provides.

### Which scenarios make sense for using Claudie and which don't?

Claudie aims to address the following scenarios, described in more detail on the [use-cases](../use-cases/use-cases.md) page:

- Cost savings
- Data locality
- Compliance (e.g. GDPR)
- Managed Kubernetes for cloud providers that do not offer it
- Cloud bursting
- Service interconnect

Using Claudie doesn't make sense when you rely on specific features of a cloud provider and necessarily tying yourself to that cloud provider.

### Is there any networking performance impact due to the introduction of the VPN layer?

We compared the use of the VPN layer with other solutions and concluded that the impact on performance is negligible.  If you are interested in performed benchmarks, we summarized the results in [our blog post](https://www.berops.com/traffic-encryption-performance-in-kubernetes-clusters/).

### What is the performance impact of a geographically distributed control plane in Claudie?

We have performed several tests and problems start to appear when the control nodes are geographically about 600 km apart. Although this is not an answer that fits all scenarios and should only be taken as a reference point.

If you are interested in the tests we have run and a more detailed answer,
you can read more in [our blog post](https://www.berops.com/evaluating-etcds-performance-in-multi-cloud/).

### Does the cloud provider traffic egress bill represent a significant part on the overall running costs?

Costs are individual and depend on the cost of the selected cloud provider and the type of workload running on the cluster based on the user's needs. Networking expenses can exceed 50% of your provider bill, therefore we recommend making your workload geography and provider aware (e.g. using taints and affinities).

### Should I be worried about giving Claudie provider credentials, including ssh keys?

Provider credentials are created as secrets in the Management Cluster for Claudie which you then reference
when creating the input manifest, that is passed to Claudie. Claudie only uses the credentials to create a connection
to nodes in the case of static nodepools or to provision the required infrastructure in the case of dynamic nodepools. The credentials are as secure as your secret management allows.

We are transparent and all of our code is open-sourced, if in doubt you can always check for yourself.

### Does each node need a public IP address?

For dynamic nodepools, nodes created by Claudie in specified cloud providers, each node needs a public IP, for static nodepools no public IP is needed.

### Is a GUI/CLI/ClusterAPI provider/Terraform provider planned?

A GUI is not actively considered at this point in time. Other possibilities are
openly discussed in [this github issue](https://github.com/berops/claudie/issues/33).

### What is the roadmap for adding support for new cloud IaaS providers?

Adding support for a new cloud provider is an easy task. Let us know your needs.
