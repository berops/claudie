<div class="hero" markdown>

# Multi-cloud Kubernetes Redefined

Build multi-cloud, hybrid-cloud, and on-premise Kubernetes clusters that mix and match nodepools across AWS, GCP, Azure, Hetzner, your own bare-metal servers, and more, managed through a single InputManifest.

[Get Started](./getting-started/get-started-using-claudie.md){ .md-button .md-button--primary }
[GitHub](https://github.com/berops/claudie){ .md-button }

[![license](https://img.shields.io/github/license/berops/claudie?style=flat-square&color=2e7da8)](https://github.com/berops/claudie/blob/master/LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/berops/claudie?style=flat-square&color=2e7da8)](https://github.com/berops/claudie)
[![Stars](https://img.shields.io/github/stars/berops/claudie?style=flat-square&color=2e7da8)](https://github.com/berops/claudie/stargazers)
[![Contributors](https://img.shields.io/github/contributors/berops/claudie?style=flat-square&color=2e7da8)](https://github.com/berops/claudie/graphs/contributors)
[![Last commit](https://img.shields.io/github/last-commit/berops/claudie?style=flat-square&color=2e7da8)](https://github.com/berops/claudie/commits)
[![Slack](https://img.shields.io/badge/Slack-join-2e7da8?style=flat-square&logo=slack)](https://kubernetes.slack.com/archives/C05SW4GKPL3)

</div>

![Claudie cluster diagram](assets/cluster-diagram-animation-light.webp#only-light)
![Claudie cluster diagram](assets/cluster-diagram-animation-dark.webp#only-dark)

## What is Claudie

Claudie is a platform for managing multi-cloud, hybrid-cloud, and on-premise Kubernetes clusters. A single cluster can mix and match nodepools from various cloud providers and your own on-premise or bare-metal servers, e.g. one nodepool in AWS, another in GCP, and a third running on hardware in your own datacenter. This is our opinionated way to build multi-cloud, hybrid-cloud, and on-premise Kubernetes infrastructure. On top of that, Claudie supports Cluster Autoscaler on the managed clusters. See the [On Premise provider docs](./input-manifest/providers/on-prem.md) for how to attach your own machines.

## Supported Providers

<div class="provider-grid" markdown>

[![AWS](https://img.shields.io/badge/AWS-232F3E?style=for-the-badge)](./input-manifest/providers/aws.md)
[![Azure](https://img.shields.io/badge/Azure-0078D4?style=for-the-badge)](./input-manifest/providers/azure.md)
[![GCP](https://img.shields.io/badge/GCP-4285F4?style=for-the-badge)](./input-manifest/providers/gcp.md)
[![Hetzner](https://img.shields.io/badge/Hetzner-D50C2D?style=for-the-badge)](./input-manifest/providers/hetzner.md)
[![OCI](https://img.shields.io/badge/OCI-F80000?style=for-the-badge)](./input-manifest/providers/oci.md)
[![Cloudflare](https://img.shields.io/badge/Cloudflare-F38020?style=for-the-badge)](./input-manifest/providers/cloudflare.md)
[![Exoscale](https://img.shields.io/badge/Exoscale-DA291C?style=for-the-badge)](./input-manifest/providers/exoscale.md)
[![OpenStack](https://img.shields.io/badge/OpenStack-ED1944?style=for-the-badge)](./input-manifest/providers/openstack.md)
[![CloudRift](https://img.shields.io/badge/CloudRift-2e7da8?style=for-the-badge)](./input-manifest/providers/cloudrift.md)
[![Verda](https://img.shields.io/badge/Verda-0E8C6B?style=for-the-badge)](./input-manifest/providers/verda.md)
[![On-Premise](https://img.shields.io/badge/On--Premise-555555?style=for-the-badge)](./input-manifest/providers/on-prem.md)

</div>

## Use cases

Claudie has been built as an answer to the following Kubernetes challenges:

* Cost savings
* Data locality & compliance (e.g. GDPR)
* Managed Kubernetes for providers that do not offer it
* Cloud bursting
* Service interconnect
* On-premise & hybrid integration - extend existing bare-metal, co-located, or private-datacenter servers into a Kubernetes cluster alongside cloud nodepools
* GPU & AI/ML workloads - Provision GPU nodes across multiple clouds, run NVIDIA GPU Operator, and leverage Cluster Autoscaler for GPU-aware scaling

You can read more [here](./use-cases/use-cases.md).

## Features

* Manage multi-cloud, hybrid-cloud, and on-premise Kubernetes clusters
* Management via IaC
* Fast scale-up/scale-down of your infrastructure
* Loadbalancing
* Persistent storage volumes
* On-premise & bare-metal nodepools - integrate your existing servers as first-class cluster nodes via SSH ([details](./input-manifest/providers/on-prem.md))
* GPU workload support - Deploy GPU-accelerated nodepools with NVIDIA GPU Operator compatibility and autoscaler-aware GPU scheduling

See more in How Claudie works sections.

!!! tip "AI/ML Ready Infrastructure"

    Claudie lets you build GPU clusters that span multiple cloud providers, combining cost-effective GPU instances from different vendors into a single Kubernetes cluster. It comes with the NVIDIA GPU Operator and Cluster Autoscaler for scalable production-ready AI/ML workloads.

!!! tip "Bring Your Own Hardware"

    Already running on-premise or co-located servers? Claudie treats them as first-class nodepools. Add them to any cluster alongside AWS, GCP, Azure, and other cloud nodepools using the [On Premise provider](./input-manifest/providers/on-prem.md), all from the same InputManifest.

## What to do next

In case you are not sure where to go next, you can just simply start with our [Getting Started Guide](./getting-started/get-started-using-claudie.md) or read our documentation [sitemap](./sitemap/sitemap.md).

If you need help or want to chat with us, feel free to join our slack channel[<a href="https://kubernetes.slack.com/archives/C05SW4GKPL3" target="_blank" rel="noopener noreferrer">
  <img src="slack_logo.png" alt="Alt text" style="height:24px;margin-left: 5px;">
</a>]()(Get invite [here](https://communityinviter.com/apps/kubernetes/community))
