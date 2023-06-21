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
  <a href="http://www.youtube.com/watch?feature=player_embedded&v=q4xdAiHYxZQ" target="_blank">
    <img src="http://img.youtube.com/vi/q4xdAiHYxZQ/0.jpg" alt="Claudie Intro Video" width="480" height="360"/>
  </a>
</p>


## Vision of Claudie

The purpose of Claudie is to become the final Kubernetes engine you'll ever need. It aims to build clusters that leverage features and costs across multiple cloud vendors and on-prem datacenters. A Kubernetes that you won't ever need to migrate away from.

## Typical use cases

Claudie has been built as an answer to the following Kubernetes challenges.

- Cost savings
- Data locality & compliance (e.g. GDPR)
- Managed Kubernetes for providers that do not offer it
- Cloud bursting
- Service interconnect

Read in more details [here](https://docs.claudie.io/latest/use-cases/use-cases/).

## Features

### Manage multi-cloud Kubernetes clusters

Create fully-featured Kubernetes clusters composed of multiple different public Cloud providers in an easy and secure manner.
Simply insert credentials to your cloud projects, define your cluster, and watch how the infra spawns right in front of you.

<p align="center">
 <img alt="Infra Diagram" src="https://github.com/berops/claudie/raw/master/docs/infra-diagram.png" />
</p>

### Management via IaC

Declaratively define your infrastructure with a simple, easy to understand YAML [syntax](https://docs.claudie.io/latest/input-manifest/input-manifest/).
See example [manifest](https://docs.claudie.io/latest/input-manifest/example/).

### Fast scale-up/scale-down of your infrastructure

To scale-up or scale-down, simply change a few lines in the input manifest and Claudie will take care of the rest in the matter of minutes.

### Loadbalancing

Claudie has its own managed load-balancing solution, which you can use for Ingresses, the Kubernetes API server, or generally anything. Check out our [LB docs](https://docs.claudie.io/latest/loadbalancing/loadbalancing-solution/).

### Persistent storage volumes

Claudie comes pre-configured with a storage solution, with ready-to-use Storage Classes. See [Storage docs](https://docs.claudie.io/latest/storage/storage-solution/) to learn more.

<!-- steps-start -->
## Get started using Claudie

### Prerequisites
Before you begin, please make sure you have the following prerequisites installed and set up:

1. Claudie needs to be installed on an existing Kubernetes cluster, which it uses to manage the clusters it provisions. For testing, you can use ephemeral clusters like Minikube or Kind. However, for production environments, we recommend using a more resilient solution since Claudie maintains the state of the infrastructure it creates.

2. Claudie requires the installation of cert-manager in your Kubernetes cluster. To install cert-manager, use the following command:
    ```bash
    kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.12.0/cert-manager.yaml
    ```


### Supported providers
<!-- providers-start -->
   | Supported Provider                                                                | Node Pools         | DNS                |
   | --------------------------------------------------------------------------------- | ------------------ | ------------------ |
   | [AWS](https://docs.claudie.io/latest/input-manifest/providers/aws/)               | :heavy_check_mark: | :heavy_check_mark: |
   | [Azure](https://docs.claudie.io/latest/input-manifest/providers/azure/)           | :heavy_check_mark: | :heavy_check_mark: |
   | [GCP](https://docs.claudie.io/latest/input-manifest/providers/gcp/)               | :heavy_check_mark: | :heavy_check_mark: |
   | [OCI](https://docs.claudie.io/latest/input-manifest/providers/oci/)               | :heavy_check_mark: | :heavy_check_mark: |
   | [Hetzner](https://docs.claudie.io/latest/input-manifest/providers/hetzner/)       | :heavy_check_mark: | :heavy_check_mark: |
   | [Cloudflare](https://docs.claudie.io/latest/input-manifest/providers/cloudflare/) | N/A                | :heavy_check_mark: |

For adding support for other cloud providers, open an issue or propose a PR.

<!-- providers-end -->

### Install Claudie

1. Download and extract Claudie manifests from our [release page](https://github.com/berops/claudie/releases):
    ```bash
    wget https://github.com/berops/claudie/releases/latest/download/claudie.zip && unzip claudie.zip -d claudie
    ```

2. Deploy Claudie into your management Kubernetes cluster:
    ```bash
    kubectl apply -k claudie
    ```

### Deploy your cluster

1. Create Kubernetes Secret resource for your provider configuration.

    ```bash
    kubectl create secret generic example-aws-secret-1 \
      --namespace=mynamespace \
      --from-literal=accesskey='myAwsAccessKey' \
      --from-literal=secretkey='myAwsSecretKey'
    ```

    Check the [supported providers](#supported-providers) for input manifest examples. For an input manifest spanning all supported hyperscalers checkout out [this example](https://docs.claudie.io/latest/input-manifest/example.md).

2. Deploy inputmanifest resource which Claudie uses to create infrastructure, include the created secret in `.spec.providers` as follows:
    ```bash
    kubectl apply -f - <<EOF
    apiVersion: claudie.io/v1beta1
    kind: InputManifest
    metadata:
      name: examplemanifest
    spec:
      providers:
          - name: aws-1
          providerType: aws
          secretRef:
              name: example-aws-secret-1 # reference the secret name
              namespace: mynamespace     # reference the secret namespace
      nodePools:
          dynamic:
          - name: control-aws
              providerSpec:
                name: aws-1
                region: eu-central-1
                zone: eu-central-1a
              count: 1
              serverType: t3.medium
              image: ami-0965bd5ba4d59211c
          - name: compute-1-aws
              providerSpec:
                name: aws-1
                region: eu-central-2
                zone: eu-central-2a
              count: 2
              serverType: t3.medium
              image: ami-0965bd5ba4d59211c
              storageDiskSize: 50
      kubernetes:
          clusters:
          - name: aws-cluster
              version: v1.24.0
              network: 192.168.2.0/24
              pools:
                control:
                    - control-aws
                compute:
                    - compute-1-aws        
    EOF
    ```
    
    ***Deleting existing inputmanifest resource deletes provisioned infrastructure!***

### Connect to your cluster
Claudie outputs base64 encoded kubeconfig secret `<cluster-name>-<cluster-hash>-kubeconfig` in the namespace where it is deployed:

1. Recover kubeconfig of your cluster by running:
    ```bash
    kubectl get secrets -n claudie -l claudie.io/output=kubeconfig -o jsonpath='{.items[0].data.kubeconfig}' | base64 -d > your_kubeconfig.yaml
    ```
2. Use your new kubeconfig:
    ```bash
    kubectl get pods -A --kubeconfig=your_kubeconfig.yaml
    ```

### Cleanup

1. To remove your cluster and its associated infrastructure, delete the cluster definition block from the inputmanifest and update the secret:
    ```bash
    kubectl apply -f - <<EOF
    apiVersion: claudie.io/v1beta1
    kind: InputManifest
    metadata:
      name: examplemanifest
    spec:
      providers:
          - name: aws-1
          providerType: aws
          secretRef:
              name: example-aws-secret-1 # reference the secret name
              namespace: mynamespace     # reference the secret namespace
      nodePools:
          dynamic:
          - name: control-aws
              providerSpec:
                name: aws-1
                region: eu-central-1
                zone: eu-central-1a
              count: 1
              serverType: t3.medium
              image: ami-0965bd5ba4d59211c
          - name: compute-1-aws
              providerSpec:
                name: aws-1
                region: eu-central-2
                zone: eu-central-2a
              count: 2
              serverType: t3.medium
              image: ami-0965bd5ba4d59211c
              storageDiskSize: 50
      kubernetes:
        clusters:
    #      - name: aws-cluster
    #          version: v1.24.0
    #          network: 192.168.2.0/24
    #          pools:
    #            control:
    #                - control-aws
    #            compute:
    #                - compute-1-aws         
    EOF
    ```
2. To delete all clusters defined in the input manifest, delete the secret. This triggers the deletion process, removing the infrastructure and all data associated with the manifest.

    ```bash
    kubectl delete inputmanifest examplemanifest
    ```
<!-- steps-end -->

## Get involved

<!-- Contributor guidelines -->
Everyone is more than welcome to open an issue, a PR or to start a discussion.

For more information about contributing please read the [contribution guidelines](https://docs.claudie.io/latest/contributing/contributing/).

If you want to have a chat with us, feel free to join our [Slack workspace](https://join.slack.com/t/claudie-workspace/shared_invite/zt-1imfso8r4-xwrpZjL9kt61FT1LjvWD5w).

## Security

While we strive to create secure software, there is always a chance that we
miss something.
If you've discovered something that requires our attention, see [our security
policy](SECURITY.md) to learn how to proceed.

## Roadmap
<!-- Add a roadmap for claudie so users know which features are being worked on and which will in future -->
To see the vision behind Claudie, please refer to the [roadmap](https://docs.claudie.io/latest/roadmap/roadmap/) document.


## Reach out to us

Claudie is proudly developed by Berops.
Feel free to request a demo [here](mailto:claudie-demo&commat;berops&period;com).
For information on enterprise support, contact us [here](mailto:claudie&commat;berops&period;com).

## LICENSE

Apache-2.0 (see [LICENSE](LICENSE) for details).
