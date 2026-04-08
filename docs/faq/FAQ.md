# Frequently Asked Questions

Here are answers to some of the questions we get asked most often.

### Does Claudie make sense as a pure K8s orchestration on a single cloud-provider IaaS?

Claudie is built for multicloud, so running it on a single provider comes with some trade-offs — for example, every node needs a public IPv4 address. That said, it works well in single-provider mode and still gives you benefits like a built-in autoscaler and the flexibility to expand or burst to additional cloud providers in the future.

### When should I use Claudie, and when shouldn't I?

Claudie is a good fit for the following scenarios (described in more detail on the [use-cases](../use-cases/use-cases.md) page):

- Cost savings
- GPUs from neocloud
- Data locality
- Compliance (e.g. GDPR)
- Managed Kubernetes for cloud providers that don't offer it
- Cloud bursting
- Service interconnect

Claudie is probably not the right choice if you're running on a single cloud provider that already offers managed Kubernetes and you're happy to accept that lock-in — for example, because you want to take advantage of that provider's unique value proposition.

### Does the VPN layer affect networking performance?

We benchmarked our VPN layer against other solutions and found the performance impact to be negligible. For details, see [our blog post](https://www.berops.com/traffic-encryption-performance-in-kubernetes-clusters/).

### How does a geographically distributed control plane affect performance?

In our tests, we started seeing issues when control plane nodes were roughly 600 km or more apart. That said, your mileage may vary — treat this as a reference point rather than a hard rule.

For a deeper dive into our test setup and results, see [our blog post](https://www.berops.com/evaluating-etcds-performance-in-multi-cloud/).

### How much do cloud egress charges add to the overall cost?

Typically, the networking expenses account for ~10% of your provider bill. However, this number can vary significantly depending on your selection of cloud providers and can range between 0% and 50%, so we suggest checking beforehand. In general, we recommend making your workloads geography- and provider-aware (e.g. with taints and affinities). We calculated some egress cost impact in this [blog post](https://claudie.io/egress-traffic-in-multi-cloud-kubernetes-do-i-need-to-worry/).

### Is it safe to give Claudie my provider credentials and SSH keys?

Your credentials are stored as Kubernetes secrets in the Management Cluster and referenced from the input manifest. Claudie only uses them to connect to existing nodes (static nodepools) or to provision new infrastructure (dynamic nodepools), so they're as secure as your cluster's secret management allows.

All of our code is open source — if you have any concerns, you can always audit it yourself.

### Does every node need a public IP address?

Yes, for dynamic nodepools (nodes provisioned by Claudie in the cloud) every node needs a public IP. Static nodepools don't require one.

### Are there plans for a GUI, CLI, ClusterAPI provider, or Terraform provider?

A GUI isn't on the roadmap right now. Other options are being discussed in [this GitHub issue](https://github.com/berops/claudie/issues/33).

### How hard is it to add support for a new cloud provider?

Adding a new provider is straightforward. If you need one that isn't supported yet, let us know.
