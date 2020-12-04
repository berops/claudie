apiVersion: kubeone.io/v1beta1
kind: KubeOneCluster
name: demo-cluster

versions:
  kubernetes: '{{ .Cluster.KubernetesVersion }}'

clusterNetwork:
  cni:
    external: {}

cloudProvider:
  none: {}
  external: false

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

machineController:
  deploy: false