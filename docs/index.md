# What is Claudie

Claudie is a platform for managing multi-cloud and hybrid-cloud Kubernetes clusters. These Kubernetes clusters can mix and match nodepools from various cloud providers, e.g. a single cluster can have a nodepool in AWS, another in GCP and another one on-premises. This is our opinionated way to build multi-cloud and hybrid-cloud Kubernetes infrastructure. On top of that Claudie supports Cluster Autoscaler on the managed clusters.

## Vision

The purpose of Claudie is to become the final Kubernetes engine you'll ever need. It aims to build clusters that leverage features and costs across multiple cloud vendors and on-prem datacenters. A Kubernetes that you won't ever need to migrate away from.

## Use cases

Claudie has been built as an answer to the following Kubernetes challenges:

* Cost savings
* Data locality & compliance (e.g. GDPR)
* Managed Kubernetes for providers that do not offer it
* Cloud bursting
* Service interconnet
* GPU & AI/ML workloads - Provision GPU nodes across multiple clouds, run NVIDIA GPU Operator, and leverage Cluster Autoscaler for GPU-aware scaling

You can read more [here](./use-cases/use-cases.md).

## Features

Claudie covers you with the following features functionalities:

* Manage multi-cloud and hybrid-cloud Kubernetes clusters
* Management via IaC
* Fast scale-up/scale-down of your infrastructure
* Loadbalancing
* Persistent storage volumes
* GPU workload support - Deploy GPU-accelerated nodepools with NVIDIA GPU Operator compatibility and autoscaler-aware GPU scheduling

See more in How Claudie works sections.

!!! tip "AI/ML Ready Infrastructure"

    Claudie lets you build GPU clusters that span multiple cloud providers, combining cost-effective GPU instances from different vendors into a single Kubernetes cluster. It comes with the NVIDIA GPU Operator and Cluster Autoscaler for scalable production-ready AI/ML workloads.

## What to do next

In case you are not sure where to go next, you can just simply start with our [Getting Started Guide](./getting-started/get-started-using-claudie.md) or read our documentation [sitemap](./sitemap/sitemap.md).

If you need help or want to chat with us, feel free to join our slack channel[<a href="https://kubernetes.slack.com/archives/C05SW4GKPL3" target="_blank" rel="noopener noreferrer">
  <img src="slack_logo.png" alt="Alt text" style="height:24px;margin-left: 5px;">
</a>]()(Get invite [here](https://communityinviter.com/apps/kubernetes/community))


