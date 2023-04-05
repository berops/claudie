apiVersion: kubeone.k8c.io/v1beta2
kind: KubeOneCluster
name: cluster

versions:
  kubernetes: '{{ .Kubernetes }}'

clusterNetwork:
  cni:
    external: {}

cloudProvider:
  none: {}
  external: false

addons:
  enable: true
  # In case when the relative path is provided, the path is relative
  # to the KubeOne configuration file.
  path: "../../addons"

apiEndpoint:
  host: '{{ .APIEndpoint }}'
  port: 6443

{{- $privateKey := "./private.pem" }}
controlPlane:
  hosts:
{{- range $nodepool := .Nodepools }}
  {{- range $nodeInfo := $nodepool.Nodes }}
    {{- if ge $nodeInfo.Node.NodeType 1}}
  - publicAddress: '{{ $nodeInfo.Node.Public }}'
    privateAddress: '{{ $nodeInfo.Node.Private }}'
    sshUsername: root
    sshPrivateKeyFile: '{{ $privateKey }}'
    hostname: '{{ $nodeInfo.Name }}'
    taints:
    - key: "node-role.kubernetes.io/master"
      effect: "NoSchedule"
    labels: 
      topology.kubernetes.io/region: '{{ $nodepool.Region }}'
      topology.kubernetes.io/zone: '{{ $nodepool.Zone }}'
      claudie.io/nodepool: '{{ $nodepool.NodepoolName }}'
      claudie.io/provider: '{{ $nodepool.CloudProviderName }}'
      claudie.io/provider-instance: '{{ $nodepool.ProviderName }}'
      claudie.io/node-type: 'control'
    {{- end}}
  {{- end}}
{{- end}}

staticWorkers:
  hosts:
{{- range $nodepool := .Nodepools }}
  {{- range $nodeInfo := $nodepool.Nodes }}
    {{- if eq $nodeInfo.Node.NodeType 0}}
  - publicAddress: '{{ $nodeInfo.Node.Public }}'
    privateAddress: '{{ $nodeInfo.Node.Private }}'
    sshUsername: root
    sshPrivateKeyFile: '{{ $privateKey }}'
    hostname: '{{ $nodeInfo.Name }}'
    labels: 
      topology.kubernetes.io/region: '{{ $nodepool.Region }}'
      topology.kubernetes.io/zone: '{{ $nodepool.Zone }}'
      claudie.io/nodepool: '{{ $nodepool.NodepoolName }}'
      claudie.io/provider: '{{ $nodepool.CloudProviderName }}'
      claudie.io/provider-instance: '{{ $nodepool.ProviderName }}'
      claudie.io/node-type: 'compute'
    {{- end}}
  {{- end}}
{{- end}}

machineController:
  deploy: false
