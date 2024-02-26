``` yaml title="example.yaml"
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: ExampleManifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  # Providers field is used for defining the providers. 
  # It is referencing a secret resource in Kubernetes cluster.
  # Each provider haves its own mandatory fields that are defined in the secret resource.
  # Every supported provider has an example in this input manifest.
  # providers:
  #   - name: 
  #       providerType:   # Type of the provider secret [aws|azure|gcp|oci|hetzner|hetznerdns|cloudflare]. 
  #       secretRef:      # Secret reference specification.
  #         name:         # Name of the secret resource.
  #         namespace:    # Namespace of the secret resource.
  providers:
    # Hetzner DNS provider.
    - name: hetznerdns-1
      providerType: hetznerdns
      secretRef:
        name: hetznerdns-secret-1
        namespace: example-namespace

    # Cloudflare DNS provider.
    - name: cloudflare-1
      providerType: cloudflare
      secretRef:
        name: cloudflare-secret-1
        namespace: example-namespace

    # Hetzner Cloud provider.
    - name: hetzner-1
      providerType: hetzner
      secretRef:
        name: hetzner-secret-1
        namespace: example-namespace

    # GCP cloud provider.
    - name: gcp-1
      providerType: gcp
      secretRef:
        name: gcp-secret-1
        namespace: example-namespace

    # OCI cloud provider.
    - name: oci-1
      providerType: oci
      secretRef:
        name: oci-secret-1
        namespace: example-namespace

    # AWS cloud provider.
    - name: aws-1
      providerType: aws
      secretRef:
        name: aws-secret-1
        namespace: example-namespace

    # Azure cloud provider.
    - name: azure-1
      providerType: azure
      secretRef:
        name: azure-secret-1
        namespace: example-namespace


  # Nodepools field is used for defining the nodepool specification.
  # You can think of them as a blueprints, not actual nodepools that will be created.
  nodePools:
    # Dynamic nodepools are created by Claudie, in one of the cloud providers specified.
    # Definition specification:
    # dynamic:
    #   - name:             # Name of the nodepool, which is used as a reference to it. Needs to be unique.
    #     providerSpec:     # Provider specification for this nodepool.
    #       name:           # Name of the provider instance, referencing one of the providers define above.
    #       region:         # Region of the nodepool.
    #       zone:           # Zone of the nodepool.
    #     count:            # Static number of nodes in this nodepool.
    #     serverType:       # Machine type of the nodes in this nodepool.
    #     image:            # OS image of the nodes in the nodepool.
    #     storageDiskSize:  # Disk size of the storage disk for compute nodepool. (optional)
    #     autoscaler:       # Autoscaler configuration. Mutually exclusive with Count.
    #       min:            # Minimum number of nodes in nodepool.
    #       max:            # Maximum number of nodes in nodepool.
    #     labels:           # Map of custom user defined labels for this nodepool. This field is optional and is ignored if used in Loadbalancer cluster. (optional)
    #     annotations:      # Map of user defined annotations, which will be applied on every node in the node pool. (optional)
    #     taints:           # Array of custom user defined taints for this nodepool. This field is optional and is ignored if used in Loadbalancer cluster. (optional)
    #       - key:          # The taint key to be applied to a node.
    #         value:        # The taint value corresponding to the taint key.
    #         effect:       # The effect of the taint on pods that do not tolerate the taint.
    #
    # Example definitions for each provider
    dynamic:
      - name: control-hetzner
        providerSpec:
          name: hetzner-1
          region: hel1
          zone: hel1-dc2
        count: 3
        serverType: cpx11
        image: ubuntu-22.04
        labels:
          country: finland
          city: helsinki
        annotations:
          node.longhorn.io/default-node-tags: '["finland"]'
        taints:
          - key: country
            value: finland
            effect: NoSchedule

      - name: compute-hetzner
        providerSpec:
          name: hetzner-1
          region: hel1
          zone: hel1-dc2
        count: 2
        serverType: cpx11
        image: ubuntu-22.04
        storageDiskSize: 50
        labels:
          country: finland
          city: helsinki
        annotations:
          node.longhorn.io/default-node-tags: '["finland"]'

      - name: compute-hetzner-autoscaled
        providerSpec:
          name: hetzner-1
          region: hel1
          zone: hel1-dc2
        serverType: cpx11
        image: ubuntu-22.04
        storageDiskSize: 50
        autoscaler:
          min: 1
          max: 5
        labels:
          country: finland
          city: helsinki
        annotations:
          node.longhorn.io/default-node-tags: '["finland"]'

      - name: control-gcp
        providerSpec:
          name: gcp-1
          region: europe-west1
          zone: europe-west1-c
        count: 3
        serverType: e2-medium
        image: ubuntu-os-cloud/ubuntu-2204-jammy-v20221206
        labels:
          country: germany
          city: frankfurt
        annotations:
          node.longhorn.io/default-node-tags: '["germany"]'

      - name: compute-gcp
        providerSpec:
          name: gcp-1
          region: europe-west1
          zone: europe-west1-c
        count: 2
        serverType: e2-small
        image: ubuntu-os-cloud/ubuntu-2204-jammy-v20221206
        storageDiskSize: 50
        labels:
          country: germany
          city: frankfurt
        taints:
          - key: city
            value: frankfurt
            effect: NoExecute
        annotations:
          node.longhorn.io/default-node-tags: '["germany"]'

      - name: control-oci
        providerSpec:
          name: oci-1
          region: eu-milan-1
          zone: hsVQ:EU-MILAN-1-AD-1
        count: 3
        serverType: VM.Standard2.1
        image: ocid1.image.oc1.eu-frankfurt-1.aaaaaaaavvsjwcjstxt4sb25na65yx6i34bzdy5oess3pkgwyfa4hxmzpqeq

      - name: compute-oci
        providerSpec:
          name: oci-1
          region: eu-milan-1
          zone: hsVQ:EU-MILAN-1-AD-1
        count: 2
        serverType: VM.Standard2.1
        image: ocid1.image.oc1.eu-frankfurt-1.aaaaaaaavvsjwcjstxt4sb25na65yx6i34bzdy5oess3pkgwyfa4hxmzpqeq
        storageDiskSize: 50

      - name: control-aws
        providerSpec:
          name: aws-1
          region: eu-central-1
          zone: eu-central-1c
        count: 2
        serverType: t3.medium
        image: ami-0965bd5ba4d59211c

      - name: compute-aws
        providerSpec:
          name: aws-1
          region: eu-central-1
          zone: eu-central-1c
        count: 2
        serverType: t3.medium
        image: ami-0965bd5ba4d59211c
        storageDiskSize: 50

      - name: control-azure
        providerSpec:
          name: azure-1
          region: West Europe
          zone: "1"
        count: 2
        serverType: Standard_B2s
        image: Canonical:0001-com-ubuntu-minimal-jammy:minimal-22_04-lts:22.04.202212120

      - name: compute-azure
        providerSpec:
          name: azure-1
          region: West Europe
          zone: "1"
        count: 2
        serverType: Standard_B2s
        image: Canonical:0001-com-ubuntu-minimal-jammy:minimal-22_04-lts:22.04.202212120
        storageDiskSize: 50

      - name: loadbalancer-1
        provider:
        providerSpec:
          name: gcp-1
          region: europe-west1
          zone: europe-west1-c
        count: 2
        serverType: e2-small
        image: ubuntu-os-cloud/ubuntu-2004-focal-v20220610

      - name: loadbalancer-2
        providerSpec:
          name: hetzner-1
          region: hel1
          zone: hel1-dc2
        count: 2
        serverType: cpx11
        image: ubuntu-20.04

    # Static nodepools are created by user beforehand.
    # Definition specification:
    # static:
    #   - name:             # Name of the nodepool, which is used as a reference to it. Needs to be unique.
    #     nodes:            # List of nodes which will be access under this nodepool.
    #       - endpoint:     # IP under which Claudie will access this node. Can be private as long as Claudie will be able to access it.
    #         secretRef:    # Secret reference specification, holding private key which will be used to SSH into the node (as root).
    #           name:       # Name of the secret resource.
    #           namespace:  # Namespace of the secret resource.
    #     labels:           # Map of custom user defined labels for this nodepool. This field is optional and is ignored if used in Loadbalancer cluster. (optional)
    #     annotations:      # Map of user defined annotations, which will be applied on every node in the node pool. (optional)
    #     taints:           # Array of custom user defined taints for this nodepool. This field is optional and is ignored if used in Loadbalancer cluster. (optional)
    #       - key:          # The taint key to be applied to a node.
    #         value:        # The taint value corresponding to the taint key.
    #         effect:       # The effect of the taint on pods that do not tolerate the taint.
    #
    # Example definitions
    static:
      - name: datacenter-1
        nodes:
          - endpoint: "192.168.10.1"
            secretRef:
              name: datacenter-1-key
              namespace: example-namespace

          - endpoint: "192.168.10.2"
            secretRef:
              name: datacenter-1-key
              namespace: example-namespace

          - endpoint: "192.168.10.3"
            secretRef:
              name: datacenter-1-key
              namespace: example-namespace
        labels:
          datacenter: datacenter-1
        annotations:
          node.longhorn.io/default-node-tags: '["datacenter-1"]'   
        taints:
          key: datacenter
          effect: NoExecute
            

  # Kubernetes field is used to define the kubernetes clusters.
  # Definition specification:
  #
  # clusters:
  #   - name:           # Name of the cluster. The name will be appended to the created node name.
  #     version:        # Kubernetes version in semver scheme, must be supported by KubeOne.
  #     network:        # Private network IP range.
  #     pools:          # Nodepool names which cluster will be composed of. User can reuse same nodepool specification on multiple clusters.
  #       control:      # List of nodepool names, which will be used as control nodes.
  #       compute:      # List of nodepool names, which will be used as compute nodes.
  #
  # Example definitions:
  kubernetes:
    clusters:
      - name: dev-cluster
        version: v1.26.0
        network: 192.168.2.0/24
        pools:
          control:
            - control-hetzner
            - control-gcp
          compute:
            - compute-hetzner
            - compute-gcp
            - compute-azure

      - name: prod-cluster
        version: v1.26.0
        network: 192.168.2.0/24
        pools:
          control:
            - control-hetzner
            - control-gcp
            - control-oci
            - control-aws
            - control-azure
          compute:
            - compute-hetzner
            - compute-gcp
            - compute-oci
            - compute-aws
            - compute-azure

      - name: hybrid-cluster
        version: v1.26.0
        network: 192.168.2.0/24
        pools:
          control:
            - datacenter-1
          compute:
            - compute-hetzner
            - compute-gcp
            - compute-azure

  # Loadbalancers field defines loadbalancers used for the kubernetes clusters and roles for the loadbalancers.
  # Definition specification for role:
  #
  # roles:
  #   - name:         # Name of the role, used as a reference later. Must be unique.
  #     protocol:     # Protocol, this role will use.
  #     port:         # Port, where traffic will be coming.
  #     targetPort:   # Port, where loadbalancer will forward traffic to.
  #     target:       # Targeted nodes on kubernetes cluster. Can be "k8sControlPlane", "k8sComputePlane" or "k8sAllNodes".
  #
  # Definition specification for loadbalancer:
  #
  # clusters:
  #   - name:         # Loadbalancer cluster name
  #     roles:        # List of role names this loadbalancer will fulfil.
  #     dns:          # DNS specification, where DNS records will be created.
  #       dnsZone:    # DNS zone name in your provider.
  #       provider:   # Provider name for the DNS.
  #       hostname:   # Hostname for the DNS record. Keep in mind the zone will be included automatically. If left empty the Claudie will create random hash as a hostname.
  #     targetedK8s:  # Name of the targeted kubernetes cluster
  #     pools:        # List of nodepool names used for loadbalancer
  #
  # Example definitions:
  loadBalancers:
    roles:
      - name: apiserver
        protocol: tcp
        port: 6443
        targetPort: 6443
        target: k8sControlPlane

    clusters:
      - name: apiserver-lb-dev
        roles:
          - apiserver
        dns:
          dnsZone: dns-zone
          provider: hetznerdns-1
        targetedK8s: dev-cluster
        pools:
          - loadbalancer-1
      - name: apiserver-lb-prod
        roles:
          - apiserver
        dns:
          dnsZone: dns-zone
          provider: cloudflare-1
          hostname: my.fancy.url
        targetedK8s: prod-cluster
        pools:
          - loadbalancer-2
```
