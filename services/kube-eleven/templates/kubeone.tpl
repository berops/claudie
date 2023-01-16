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

controlPlane:
  hosts:
{{- $privateKey := "./private.pem" }}
{{- range $nodeInfo := .Nodes }}
{{- if ge $nodeInfo.Node.NodeType 1}}
  - publicAddress: '{{ $nodeInfo.Node.Public }}'
    privateAddress: '{{ $nodeInfo.Node.Private }}'
    sshUsername: root
    sshPrivateKeyFile: '{{ $privateKey }}'
    hostname: '{{ $nodeInfo.Name }}'
    taints:
    - key: "node-role.kubernetes.io/master"
      effect: "NoSchedule"
{{- end}}
{{- end}}

staticWorkers:
  hosts:
{{- range $nodeInfo := .Nodes }}
{{- if eq $nodeInfo.Node.NodeType 0}}
  - publicAddress: '{{ $nodeInfo.Node.Public }}'
    privateAddress: '{{ $nodeInfo.Node.Private }}'
    sshUsername: root
    sshPrivateKeyFile: '{{ $privateKey }}'
    hostname: '{{ $nodeInfo.Name }}'
{{- end}}
{{- end}}

machineController:
  deploy: false
