# Detailed guide
This detailed guide for Claudie serves as a resource for providing an overview of Claudie's features, installation instructions, customization options, and its role in provisioning and managing clusters. We'll start by guiding you through the process of setting up a management cluster, where Claudie will be installed, enabling you to effortlessly monitor and control clusters across multiple hyperscalers.

!!! note "Tip!"
    Claudie offers extensive customization options for your Kubernetes cluster across multiple hyperscalers. This detailed guide assumes you have AWS and Hetzner accounts. You can customize your deployment across different [supported providers](#supported-providers). If you wish to use different providers, we recommend to follow this guide anyway and create your own input manifest file based on the provided example. Refer to the supported provider table for the input manifest configuration of each provider.

## Supported providers
{%
   include-markdown "../../README.md"
   start="<!-- providers-start -->"
   end="<!-- providers-end -->"
%}

## Prerequisites
1. Install Kind by following the [Kind documentation](https://kind.sigs.k8s.io/docs/user/quick-start/#installation).
2. Install kubectl tool to communicate with your management cluster by following the [Kubernetes documentation](https://kubernetes.io/docs/tasks/tools/#kubectl). 
3. Install Kustomize by following [Kustomize documentation](https://kubectl.docs.kubernetes.io/installation/kustomize/).
4. Install Docker by following [Docker documentation](https://docs.docker.com/engine/install/).

## Claudie deployment

1. Create a Kind cluster where you will deploy Claudie, also called management cluster.

    ```bash
    kind create cluster --name=claudie
    ```

    !!! note "Management cluster consideration."
        We recommend using a non-ephemeral management cluster! Deleting the management cluster prevents autoscaling of Claudie node pools as well as loss of state! We recommended to use a managed Kubernetes offerings to ensure management cluster resiliency. Kind cluster is sufficient for this guide.

2. Check if have the correct current kubernetes context. The context should be `kind-claudie`.

    ```bash
    kubectl config current-context
    ```

3.  If context is not `kind-claudie`, switch to it:

    ```bash
    kubectl config use-context kind-claudie
    ```

4. One of the prerequisites is `cert-manager`, deploy it with the following command:

    ```bash
    kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.12.0/cert-manager.yaml
    ```

5. Download latest Claudie release and unzip it:

    ```bash
    wget https://github.com/berops/claudie/releases/latest/download/claudie.zip && unzip claudie.zip -d claudie
    ```

    !!! note "Tip!"  
        For the initial attempt, it's highly recommended to enable debug logs, especially when creating a large cluster with DNS. This helps identify and resolve any permission issues that may occur across different hyperscalers. Edit `claudie/.env` file, and change `GOLANG_LOG=info` to `GOLANG_LOG=debug` to enable debug logging, for more customization refer to [this table](#claudie-customization).

6. Deploy Claudie using Kustomize plugin:
    ```bash
    kubectl apply -k claudie
    ```

7. Claudie will be deployed into `claudie` namespace, you can view if all pods are running:

    ```bash
    kubectl get pods -n claudie 
    ```
    ```text
    NAME                           READY   STATUS      RESTARTS        AGE
    ansibler-5c6c776b75-82c2q      1/1     Running     0               8m10s
    builder-59f9d44596-n2qzm       1/1     Running     0               8m10s
    context-box-5d76c89b4d-tb6h4   1/1     Running     1 (6m37s ago)   8m10s
    create-table-job-jvs9n         0/1     Completed   1               8m10s
    dynamodb-68777f9787-8wjhs      1/1     Running     0               8m10s
    frontend-5755b7bc69-5l84h      1/1     Running     0               8m10s
    kube-eleven-64468cd5bd-qp4d4   1/1     Running     0               8m10s
    kuber-698c4564c-dhsvg          1/1     Running     0               8m10s
    make-bucket-job-fb5sp          0/1     Completed   0               8m10s
    minio-0                        1/1     Running     0               8m10s
    minio-1                        1/1     Running     0               8m10s
    minio-2                        1/1     Running     0               8m10s
    minio-3                        1/1     Running     0               8m10s
    mongodb-67bf769957-9ct5z       1/1     Running     0               8m10s
    scheduler-654cbd4b97-qwtbf     1/1     Running     0               8m10s
    terraformer-fd664b7ff-dd2h7    1/1     Running     0               8m9s
    ```

    !!! warning "Troubleshoot!" 
        If you experience problems refer to our [troubleshooting guide](https://docs.claudie.io/latest/troubleshooting/troubleshooting/). 

8. Let's create a AWS high availability cluster which we'll expand later on with Hetzner bursting capacity. Let's start by creating providers secrets for the infrastructure, and next we will reference them in `inputmanifest-bursting.yaml`.

    ```bash
    # AWS provider requires the secrets to have fields: accesskey and secretkey
    kubectl create secret generic aws-secret-1 --namespace=mynamespace --from-literal=accesskey='SLDUTKSHFDMSJKDIALASSD' --from-literal=secretkey='iuhbOIJN+oin/olikDSadsnoiSVSDsacoinOUSHD'
    kubectl create secret generic aws-secret-dns --namespace=mynamespace --from-literal=accesskey='ODURNGUISNFAIPUNUGFINB' --from-literal=secretkey='asduvnva+skd/ounUIBPIUjnpiuBNuNipubnPuip'    
    ```

    ```yaml
    # inputmanifest-bursting.yaml

    apiVersion: claudie.io/v1beta1
    kind: InputManifest
    metadata:
      name: cloud-bursting
    spec:
      providers:
        - name: aws-1
          providerType: aws
          secretRef:
            name: aws-secret-1
            namespace: mynamespace
        - name: aws-dns
          providerType: aws
          secretRef:
            name: aws-secret-dns
            namespace: mynamespace    
      nodePools:
        dynamic:
          - name: aws-controlplane
            providerSpec:
                name: aws-1
                region: eu-central-1
                zone: eu-central-1a
            count: 3
            serverType: t3.medium
            image: ami-0965bd5ba4d59211c
          - name: aws-worker
            providerSpec:
                name: aws-1
                region: eu-north-1
                zone: eu-north-1a
            count: 3
            serverType: t3.medium
            image: ami-03df6dea56f8aa618
            storageDiskSize: 200
          - name: aws-loadbalancer
            providerSpec:
                name: aws-1
                region: eu-central-2
                zone: eu-central-2a
            count: 2
            serverType: t3.small
            image: ami-0965bd5ba4d59211c
      kubernetes:
        clusters:
          - name: my-super-cluster
            version: v1.24.0
            network: 192.168.2.0/24
            pools:
                control:
                - aws-controlplane
                compute:
                - aws-worker
      loadBalancers:
        roles:
          - name: apiserver
            protocol: tcp
            port: 6443
            targetPort: 6443
            target: k8sControlPlane
        clusters:
          - name: loadbalance-me
            roles:
                - apiserver
            dns:
                dnsZone: domain.com # hosted zone domain name where claudie creates dns records for this cluster
                provider: aws-dns
                hostname: supercluster # the sub domain of the new cluster
            targetedK8s: my-super-cluster
            pools:
                - aws-loadbalancer
    ```

    !!! note "Tip!"
        In this example, two AWS providers are used — one with access to compute resources and the other with access to DNS. However, it is possible to use a single AWS provider with permissions for both services.

9. Apply the `inputmanifest` crd with your cluster configuration file:

    ```bash
    kubectl apply -f ./inputmanifest-bursting.yaml
    ```

    !!! note "Tip!"
        Inputmanifests serve as a single source of truth for both Claudie and the user, which makes creating infrastructure via input manifests as infrastructure as a code and can be easily integrated into a GitOps workflow.

    !!! warning "Errors in input manifest"
        Validation webhook will reject the inputmanifest at this stage if it finds errors within the manifest. Refer to our [API guide](https://docs.claudie.io/latest/input-manifest/api-reference/) for details.

11. View logs from `frontend` service to see secret picked up, as well as which service is currently doing the work:

    View the `inputmanifest` state with `kubectl`

    ```bash
    kubectl get inputmanifests.claudie.io cloud-bursting -o jsonpath={.status} | jq .
    ```
    Here’s an example of what frontend might output at this point:

    ```json
      {
        "clusters": {
          "my-super-cluster": {
            "message": " installing VPN",
            "phase": "ANSIBLER",
            "state": "IN_PROGRESS"
          }
        },
        "state": "IN_PROGRESS"
      }
    ```

    !!! note "Claudie architecture"
        Claudie utilizes multiple services for cluster provisioning, refer to our [workflow documentation](https://docs.claudie.io/latest/claudie-workflow/claudie-workflow/) as to how it works under the hood. Frontend's consolidated log provides visibility into the ongoing operations of each individual Claudie service. 

    !!! warning "Provisioning times may vary!"
        Please note that cluster creation time may vary due to provisioning capacity and machine provisioning times of selected hyperscalers.

    After finishing the `inputmanifest` state reflects that the cluster is provisioned.

    ```json
    kubectl get inputmanifests.claudie.io cloud-bursting -o jsonpath={.status} | jq .
      {
        "clusters": {
          "my-super-cluster": {
            "phase": "NONE",
            "state": "DONE"
          }
        },
        "state": "DONE"
      }    
    ```

12. Claudie creates kubeconfig secret in claudie namespace:

    ```bash
    kubectl get secrets -l claudie.io/output=kubeconfig
    ```
    ```
    NAME                                  TYPE     DATA   AGE
    my-super-cluster-6ktx6rb-kubeconfig   Opaque   1      134m
    ```

    You can recover kubeconfig for your cluster with the following command:

    ```bash
    kubectl get secrets -n claudie -l claudie.io/output=kubeconfig -o jsonpath='{.items[0].data.kubeconfig}' | base64 -d > my-super-cluster-kubeconfig.yaml
    ```

    If you want to connect to your machines via SSH, you can recover private SSH key:

    ```bash
    kubectl get secrets -n claudie -l claudie.io/output=metadata -ojsonpath='{.items[0].data.metadata}' | base64 -d | jq -r .private_key > ~/.ssh/my-super-cluster
    ```

    To recover public IP of a node to connect to via SSH:
    ```bash
    kubectl get secrets -n claudie -l claudie.io/output=metadata -ojsonpath='{.items[0].data.metadata}' | base64 -d | jq -r .node_ips
    ```

    Each secret created by Claudie has following labels:

    | Key                     | Value                                           |
    | ----------------------- | ----------------------------------------------- |
    | `claudie.io/project`    | Name of the project.                            |
    | `claudie.io/cluster`    | Name of the cluster.                            |
    | `claudie.io/cluster-id` | ID of the cluster.                              |
    | `claudie.io/output`     | Output type, either `kubeconfig` or `metadata`. |


13. Use your new kubeconfig to see what’s in your new cluster

    ```bash
    kubectl get pods -A --kubeconfig=my-super-cluster-kubeconfig.yaml
    ```

14. Let's add a bursting autoscaling node pool in Hetzner cloud. In order to use other hyperscalers, we'll need to add a new provider with appropriate credentials. First we will create a provider secret for Hetzner Cloud, then we open `inputmanifest-bursting.yaml` input manifest again and append the new Hetzner node pool configuration.

    ```bash
    # Hetzner provider requires the secrets to have field: credentials
    kubectl create secret generic hetzner-secret-1 --namespace=mynamespace --from-literal=credentials='kslISA878a6etYAfXYcg5iYyrFGNlCxcICo060HVEygjFs21nske76ksjKko21lp'
    ```

    !!! note "Claudie autoscaling"
        Autoscaler in Claudie is deployed in Claudie management cluster and provisions additional resources remotely at the time of need. For more information check out how [Claudie autoscaling](https://docs.claudie.io/latest/autoscaling/autoscaling.md) works.

    ```yaml
    # inputmanifest-bursting.yaml

    apiVersion: claudie.io/v1beta1
    kind: InputManifest
    metadata:
      name: cloud-bursting
    spec:
      providers:
        - name: hetzner-1         # add under nodePools.dynamic section
          providerType: hetzner
          secretRef:
            name: hetzner-secret-1
            namespace: mynamespace        
      nodePools:
        dynamic:
        ...
          - name: hetzner-worker  # add under nodePools.dynamic section
            providerSpec:
                name: hetzner-1   # use your new hetzner provider hetzner-1 to create these nodes
                region: hel1
                zone: hel1-dc2
            serverType: cpx51
            image: ubuntu-22.04
            autoscaler:           # this node pool uses a claudie autoscaler instead of static count of nodes
                min: 1
                max: 10
        kubernetes:
          clusters:
          - name: my-super-cluster
            version: v1.24.0
            network: 192.168.2.0/24
            pools:
                control:
                - aws-controlplane
                compute:
                - aws-worker
                - hetzner-worker  # add it to the compute list here
    ...
    ```

15. Update the crd with the new inputmanifest to incorporate the desired changes.

    !!! danger "Deleting existing secrets!"
        **Deleting or replacing existing input manifest secrets triggers cluster deletion!** To add new components to your existing clusters, generate a new secret value and apply it using the following command.

    ```bash
    kubectl apply -f ./inputmanifest-bursting.yaml
    ```

16. You can also passthrough additional ports from load balancers to control plane and or worker node pools by adding additional roles under `roles`.
    ```yaml
    # inputmanifest-bursting.yaml

    apiVersion: claudie.io/v1beta1
    kind: InputManifest
    metadata:
      name: cloud-bursting
    spec:
      ...
      loadBalancers:
        roles:
          - name: apiserver
            protocol: tcp
            port: 6443
            targetPort: 6443
            target: k8sControlPlane
          - name: https
            protocol: tcp
            port: 443
            targetPort: 443
            target: k8sComputeNodes # only loadbalance between workers
        clusters:
          - name: loadbalance-me
            roles:
                - apiserver
                - https # define it here
            dns:
                dnsZone: domain.com
                provider: aws-dns
                hostname: supercluster
            targetedK8s: my-super-cluster
            pools:
                - aws-loadbalancer
    ```
    !!! note Load balancing
        Please refer how our load balancing works by reading our [documentation](https://docs.claudie.io/latest/loadbalancing/loadbalancing-solution/).

17. Update the inputmanifest again with the new configuration.
    ```bash
    kubectl apply -f ./inputmanifest-bursting.yaml
    ```

18. To delete the cluster just simply delete the secret and wait for Claudie to destroy it.

    ```bash
    kubectl delete -f ./inputmanifest-bursting.yaml
    ```

    !!! warning "Removing clusters"
        Deleting Claudie or the management cluster does not remove the Claudie managed clusters. Delete the secret first to initiate Claudie's deletion process.

19. After frontend finished deletion workflow delete minikube cluster 
    ```bash
    kind delete cluster
    ```

## General tips

### Control plane considerations
- **Single Control Plane Node:** Node pool with one machine manages your cluster.
- **Multiple Control Plane Nodes:** Control plane node pool that has more than one node.
    - **Load Balancer Requirement:** A load balancer is optional for high availability setup, however we recommend it. Include an additional node pool for load balancers.
    - **DNS Requirement:** If you want to use load balancing, you will need a registered domain name, and a hosted zone. Claudie creates a failover DNS record for the load balancer machines.
        - **Supported DNS providers:** If your DNS provider is not supported, delegate a subdomain to a supported DNS provider, refer to supported DNS providers.
    - **Egress Traffic**: Hyperscalers charge for outbound data and multi-region infrastructure. To avoid egress traffic deploy control plane node pools in the same region to one hypoerscaler. If availability is more important than egress traffic costs, you can have multiple control plane node pools spanning across different hyperscalers.

### Egress traffic
Hyperscalers charge for outbound data and multi-region infrastructure.

- **Control plane:** To avoid egress traffic deploy control plane node pools in the same region to one hyperscaler. If availability is more important than egress traffic costs, you can have multiple control plane node pools spanning across different hyperscalers.

- **Workloads:** Egress costs associated with workloads are more complicated as they depend on each use case. What we recommend it to try and use localised workloads where possible. 

!!! note "Example"
    Consider a scenario where you have a workload that involves processing extensive datasets from GCP storage using Claudie managed AWS GPU instances. To minimize egress network traffic costs, it is recommended to host the datasets in an S3 bucket and limit egress traffic from GCP and keep the workload localised.

### On your own path
Once you've gained a comprehensive understanding of how Claudie operates through this guide, you can deploy it to a reliable management cluster, this could be a cluster that you already have. Tailor your input manifest file to suit your specific requirements and explore a detailed example showcasing providers, load balancing, and DNS records across various hyperscalers by visiting this [comprehensive example](https://docs.claudie.io/latest/input-manifest/example/).

## Claudie customization

All of the customisable settings can be found in `claudie/.env` file.

| Variable               | Default       | Type   | Description                                                  |
| ---------------------- | ------------- | ------ | ------------------------------------------------------------ |
| `GOLANG_LOG`           | `info`        | string | Log level for all services. Can be either `info` or `debug`. |
| `DATABASE_HOSTNAME`    | `mongodb`     | string | Database hostname used for Claudie configs.                  |
| `CONTEXT_BOX_HOSTNAME` | `context-box` | string | Context-box service hostname.                                |
| `TERRAFORMER_HOSTNAME` | `terraformer` | string | Terraformer service hostname.                                |
| `ANSIBLER_HOSTNAME`    | `ansibler`    | string | Ansibler service hostname.                                   |
| `KUBE_ELEVEN_HOSTNAME` | `kube-eleven` | string | Kube-eleven service hostname.                                |
| `KUBER_HOSTNAME`       | `kuber`       | string | Kuber service hostname.                                      |
| `MINIO_HOSTNAME`       | `minio`       | string | MinIO hostname used for state files.                         |
| `DYNAMO_HOSTNAME`      | `dynamo`      | string | DynamoDB hostname used for lock files.                       |
| `DYNAMO_TABLE_NAME`    | `claudie`     | string | Table name for DynamoDB lock files.                          |
| `AWS_REGION`           | `local`       | string | Region for DynamoDB lock files.                              |
| `DATABASE_PORT`        | 27017         | int    | Port of the database service.                                |
| `TERRAFORMER_PORT`     | 50052         | int    | Port of the Terraformer service.                             |
| `ANSIBLER_PORT`        | 50053         | int    | Port of the Ansibler service.                                |
| `KUBE_ELEVEN_PORT`     | 50054         | int    | Port of the Kube-eleven service.                             |
| `CONTEXT_BOX_PORT`     | 50055         | int    | Port of the Context-box service.                             |
| `KUBER_PORT`           | 50057         | int    | Port of the Kuber service.                                   |
| `MINIO_PORT`           | 9000          | int    | Port of the MinIO service.                                   |
| `DYNAMO_PORT`          | 8000          | int    | Port of the DynamoDB service.                                |
