<h4 align="center">
  <img src="https://raw.githubusercontent.com/berops/claudie/17480b6cb809fe795d454588af18355c7543f37e/docs/logo%20claudie_blue_no_BG.svg" width="250px"/><br/>
  <br/><br/>
  Platform for managing multi-cloud and hybrid-cloud Kubernetes clusters with support for nodepools across different cloud-providers and on-premise data centers
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

### Manage multi-cloud and hybrid-cloud Kubernetes clusters

Create fully-featured Kubernetes clusters composed of multiple different public Cloud providers and on-premise data center in an easy and secure manner.
Simply insert credentials to your cloud projects, define your cluster, and watch how the infra spawns right in front of you.

![](./docs/infra-diagram.png)

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

1. Claudie needs to be installed on an existing Kubernetes cluster, referred to as the *Management Cluster*, which it uses to manage the clusters it provisions. For testing, you can use ephemeral clusters like Minikube or Kind. However, for production environments, we recommend using a more resilient solution since Claudie maintains the state of the infrastructure it creates.

2. Claudie requires the installation of cert-manager in your Management Cluster. To install cert-manager, use the following command:
    ```bash
    kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.12.0/cert-manager.yaml
    ```


### Supported providers
<!-- providers-start -->
   | Supported Provider                                                                | Node Pools         | DNS                | DNS healthchecks  |
   | --------------------------------------------------------------------------------- | ------------------ | ------------------ |------------------ |
   | [AWS](https://docs.claudie.io/latest/input-manifest/providers/aws/)               | :heavy_check_mark: | :heavy_check_mark: |:heavy_check_mark: |
   | [Azure](https://docs.claudie.io/latest/input-manifest/providers/azure/)           | :heavy_check_mark: | :heavy_check_mark: |:heavy_check_mark: |
   | [GCP](https://docs.claudie.io/latest/input-manifest/providers/gcp/)               | :heavy_check_mark: | :heavy_check_mark: |:heavy_check_mark: |
   | [OCI](https://docs.claudie.io/latest/input-manifest/providers/oci/)               | :heavy_check_mark: | :heavy_check_mark: |:heavy_check_mark: |
   | [Hetzner](https://docs.claudie.io/latest/input-manifest/providers/hetzner/)       | :heavy_check_mark: | :heavy_check_mark: | N/A               |
   | [Cloudflare](https://docs.claudie.io/latest/input-manifest/providers/cloudflare/) | N/A                | :heavy_check_mark: |:heavy_check_mark: |
   | [GenesisCloud](https://docs.claudie.io/latest/input-manifest/providers/genesiscloud/) | :heavy_check_mark: | N/A            | N/A               |

For adding support for other cloud providers, open an issue or propose a PR.

<!-- providers-end -->

### Install Claudie

1. Deploy Claudie to the Management Cluster:
    ```bash
    kubectl apply -f https://github.com/berops/claudie/releases/latest/download/claudie.yaml
    ```

   To further harden claudie, you may want to deploy our pre-defined network policies:
   ```bash
   # for clusters using cilium as their CNI
   kubectl apply -f https://github.com/berops/claudie/releases/latest/download/network-policy-cilium.yaml
   ```
   ```bash
   # other
   kubectl apply -f https://github.com/berops/claudie/releases/latest/download/network-policy.yaml
   ```
### Deploy your cluster

1. Create Kubernetes Secret resource for your provider configuration.

    ```bash
    kubectl create secret generic example-aws-secret-1 \
      --namespace=mynamespace \
      --from-literal=accesskey='myAwsAccessKey' \
      --from-literal=secretkey='myAwsSecretKey'
    ```

    Check the [supported providers](#supported-providers) for input manifest examples. For an input manifest spanning all supported hyperscalers checkout out [this example](https://docs.claudie.io/latest/input-manifest/example/).

2. Deploy InputManifest resource which Claudie uses to create infrastructure, include the created secret in `.spec.providers` as follows:
    ```bash
    kubectl apply -f - <<EOF
    apiVersion: claudie.io/v1beta1
    kind: InputManifest
    metadata:
      name: examplemanifest
      labels:
        app.kubernetes.io/part-of: claudie
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
                region: eu-west-3
                zone: eu-west-3a
              count: 2
              serverType: t3.medium
              image: ami-029c608efaef0b395
              storageDiskSize: 50
      kubernetes:
          clusters:
          - name: aws-cluster
              version: 1.27.0
              network: 192.168.2.0/24
              pools:
                control:
                    - control-aws
                compute:
                    - compute-1-aws        
    EOF
    ```
    
    ***Deleting existing InputManifest resource deletes provisioned infrastructure!***

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

1. To remove your cluster and its associated infrastructure, delete the cluster definition block from the InputManifest:
    ```bash
    kubectl apply -f - <<EOF
    apiVersion: claudie.io/v1beta1
    kind: InputManifest
    metadata:
      name: examplemanifest
      labels:
        app.kubernetes.io/part-of: claudie
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
                region: eu-west-3
                zone: eu-west-3a
              count: 2
              serverType: t3.medium
              image: ami-029c608efaef0b395
              storageDiskSize: 50
      kubernetes:
        clusters:
    #      - name: aws-cluster
    #          version: 1.27.0
    #          network: 192.168.2.0/24
    #          pools:
    #            control:
    #                - control-aws
    #            compute:
    #                - compute-1-aws         
    EOF
    ```
2. To delete all clusters defined in the input manifest, delete the InputManifest. This triggers the deletion process, removing the infrastructure and all data associated with the manifest.

    ```bash
    kubectl delete inputmanifest examplemanifest
    ```
<!-- steps-end -->

## Get involved

<!-- Contributor guidelines -->
Everyone is more than welcome to open an issue, a PR or to start a discussion.

For more information about contributing please read the [contribution guidelines](https://docs.claudie.io/latest/contributing/contributing/).

If you want to have a chat with us, feel free to join our channel on [kubernetes Slack workspace](https://kubernetes.slack.com/archives/C05SW4GKPL3) (get invite [here](https://communityinviter.com/apps/kubernetes/community)).

## Versioning

Current project releasing follows [ZerOver](https://0ver.org), with the following versioning promise:
- In new releases, API might break and functionality might change significantly. Any such releases increment the second digit in the release tag. The users **really need to read the release notes** before upgrading to these releases.
- For all other releases, the third digit increments. Upgrades to these versions can be done blindly without any risk to running environments. Reading the release notes is recommended nevertheless.

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
