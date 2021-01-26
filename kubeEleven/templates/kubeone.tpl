apiVersion: kubeone.io/v1beta1
kind: KubeOneCluster
name: cluster

versions:
  kubernetes: '{{ .Cluster.KubernetesVersion }}'

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
  path: "../addons"

apiEndpoint:
{{- $node := index .Cluster.Nodes 0 }}
  host: '{{ $node.PublicIp }}'
  port: 6443

controlPlane:
  hosts:
{{- $privateKey := .Cluster.PrivateKey }}  
{{- range .Cluster.Nodes}}
{{- if eq .IsWorker false}}
  - publicAddress: '{{ .PublicIp }}'
    privateAddress: '{{ .PrivateIp }}'
    sshPrivateKeyFile: '{{ $privateKey }}'
{{- end}}
{{- end}}

staticWorkers:
  hosts:
{{- $privateKey := .Cluster.PrivateKey }}  
{{- range .Cluster.Nodes}}
{{- if eq .IsWorker true}}
  - publicAddress: '{{ .PublicIp }}'
    privateAddress: '{{ .PrivateIp }}'
    sshPrivateKeyFile: '{{ $privateKey }}'
{{- end}}
{{- end}}

machineController:
  deploy: false