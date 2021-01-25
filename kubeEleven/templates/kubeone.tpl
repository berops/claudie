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
{{ $privateKey := .Cluster.PrivateKey }}  
{{- range .Cluster.Nodes}}
  - publicAddress: '{{ .PublicIp }}'
    privateAddress: '{{ .PrivateIp }}'
    sshPrivateKeyFile: '{{ $privateKey }}'
{{- end}}

staticWorkers:
  hosts:
{{ $privateKey := .Cluster.PrivateKey }}  
{{- range .Cluster.Nodes}}
  - publicAddress: '{{ .PublicIp }}'
    privateAddress: '{{ .PrivateIp }}'
    sshPrivateKeyFile: '{{ $privateKey }}'
{{- end}}

# staticWorkers:
#   hosts: 
#   - publicAddress: '135.181.37.13'
#     privateAddress: '192.168.2.3'
#     sshPrivateKeyFile: '/Users/samuelstolicny/.ssh/samuelstolicny_ssh_key'

machineController:
  deploy: false