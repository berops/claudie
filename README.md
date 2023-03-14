<h4 align="center">
  <img src="https://raw.githubusercontent.com/berops/claudie/17480b6cb809fe795d454588af18355c7543f37e/docs/logo%20claudie_blue_no_BG.svg" width="250px"/><br/>
  <br/><br/>
  Platform for managing multi-cloud Kubernetes clusters with each nodepool in a different cloud-provider
</h4>

<p align="center">
  <a href="https://github.com/berops/claudie/releases/"><img alt="Releases" src="https://img.shields.io/github/release-date/berops/claudie?label=latest%20release" /></a>
  <a href="https://goreportcard.com/report/github.com/Berops/claudie"><img src="https://goreportcard.com/badge/github.com/Berops/claudie"></a>
  <a href="https://opensource.org/licenses/Apache-2.0"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg"></a>
</p>

## Intro video

<p align="center">
  <a href="https://youtu.be/q4xdAiHYxZQ"><img src="https://markdown-videos.deta.dev/youtube/q4xdAiHYxZQ"></a>
</p>

## Typical use cases

Claudie has been built to target the following use case in the Kubernetes world.

- Cloud bursting
- Service interconnect
- Managed Kubernetes for providers that do not offer it
- Cost savings

Read in more details [here](./docs/use-cases/use-cases.md).

## Features

### Manage multi-cloud Kubernetes clusters

Create fully-featured Kubernetes clusters composed of multiple different public Cloud providers in an easy and secure manner.
Simply insert credentials to your cloud projects, define your cluster, and watch how the infra spawns right in front of you.

<p align="center">
 <img alt="Infra Diagram" src="https://github.com/berops/claudie/raw/master/docs/infra-diagram.png" />
</p>

### Management via IaC

Declaratively define your infrastructure with a simple, easy to understand YAML [syntax](./docs/input-manifest/input-manifest.md).
See example [manifest](./docs/input-manifest/example.yaml).

### Fast scale-up/scale-down of your infrastructure

To scale-up or scale-down, simply change a few lines in the input manifest and Claudie will take care of the rest in the matter of minutes.

### Loadbalancing

Claudie has its own managed load-balancing solution, which you can use for Ingresses, the Kubernetes API server, or generally anything. Check out our [LB docs](https://github.com/Berops/claudie/tree/master/docs/loadbalancing).

### Persistent storage volumes

Claudie comes pre-configured with a storage solution, with ready-to-use Storage Classes. See [Storage docs](https://github.com/Berops/claudie/tree/master/docs/storage) to learn more.

### Supported cloud providers

| Cloud Provider                                                                                          | Infrastructure support | DNS support |
| ------------------------------------------------------------------------------------------------------- | ---------------------- | ----------- |
| [AWS](https://github.com/berops/claudie/blob/master/docs/input-manifest/providers/aws.md)               | Supported              | Supported   |
| [Azure](https://github.com/berops/claudie/blob/master/docs/input-manifest/providers/azure.md)           | Supported              | Supported   |
| [GCP](https://github.com/berops/claudie/blob/master/docs/input-manifest/providers/gcp.md)               | Supported              | Supported   |
| [OCI](https://github.com/berops/claudie/blob/master/docs/input-manifest/providers/oci.md)               | Supported              | Supported   |
| [Hetzner](https://github.com/berops/claudie/blob/master/docs/input-manifest/providers/hetzner.md)       | Supported              | Supported   |
| [Cloudflare](https://github.com/berops/claudie/blob/master/docs/input-manifest/providers/cloudflare.md) | Not applicable         | Supported   |

## Get started using Claudie

1. Deploy Claudie Kubernetes [manifests/claudie](https://github.com/Berops/claudie/tree/master/manifests/claudie) into a Kubernetes cluster:

   ```sh
   kubectl apply -k manifests/claudie
   ```

2. provide your own manifest via a Kubernetes Secret.

Have a look at our [reference example input manifest](https://github.com/Berops/claudie/blob/master/docs/input-manifest/example.yaml) to explore what's possible.

To see in detail how to correctly apply the manifest into Claudie, please refer to the [CRUD](./docs/crud/crud.md) document.

After the input manifest is successfully applied, the kubeconfig to your newly
built clusters is output as a secret in the `claudie` namespace with a name in
the form of `<cluster-name>-<cluster-hash>-kubeconfig`.

## Get involved

<!-- Contributor guidelines -->
Everyone is more than welcome to open an issue, a PR or to start a discussion.

For more information about contributing please read the [contribution guidelines](./docs/contributing/contributing.md).

If you want to have a chat with us, feel free to join our ["claudie-workspace" Slack workspace](https://join.slack.com/t/claudie-workspace/shared_invite/zt-1imfso8r4-xwrpZjL9kt61FT1LjvWD5w).

## Security

While we strive to create secure software, there is always a chance that we
miss something.
If you've discovered something that requires our attention, see [our security
policy](SECURITY.md) to learn how to proceed.

## Roadmap
<!-- Add a roadmap for claudie so users know which features are being worked on and which will in future -->
To see the vision behind Claudie, please refer to the [roadmap](./docs/roadmap/roadmap.md) document.

## LICENSE

Apache-2.0 (see [LICENSE](LICENSE) for details).
