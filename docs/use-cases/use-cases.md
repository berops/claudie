
# Use-cases and customers

## We foresee the following use-cases of the Claudie platform

## 1. Cloud-bursting

A company uses advanced cloud features in one of the hyper-scale providers (e.g. serverless Lambda and API Gateway functionality in AWS). They run a machine-learning application that they need to train for a pattern on a dataset. The learning phase requires significant compute resources. Claudie allows to extend the cluster in AWS (needed in order to access the AWS functionality) to Hetzner for saving the infrastructure costs of the machine-learning case.

Typical client profiles:
- startups
- in need of significant computing power already in their early stages (e.g. AI/ML workloads)

## 2. Cost-saving

A company would like to utilize their on-premise or leased resources that they already invested into, but would like to:
1. extend the capacity
2. access managed features of a hyper-scale provider (AWS, GCP, ...)
3. get the workload physically closer to a client (e. g. to South America)

Typical client profile:
- medium-size business
- possibly already familiar with containerized workload

## 3. Smart-layer-as-a-Service on top of simple cloud-providers

An existing customer of medium-size provider (e.g. Exoscale) would like to utilize features that are typical for hyper-scale providers. Their current provider does neither offer nor plan to offer such an advanced functionality.

Typical client profile:
- established business
- need to access advanced managed features to innovate faster

## 4. Service interconnect

A company would like to access on-premise-hosted services and cloud-managed services from within the same cluster. For on-premise services the on-premise cluster node would egress the traffic. The cloud-hosted cluster nodes would deal with the egress traffic to the cloud-managed services.

Typical client profile:
- medium-size/established business
- already contains on-premise workloads
- has the need to take the advantage of managed cloud infra (from cost, agility, or capacity reasons)

