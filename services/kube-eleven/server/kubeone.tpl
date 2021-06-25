apiVersion: kubeone.io/v1beta1
kind: KubeOneCluster
name: cluster

versions:
  kubernetes: '{{ .Cluster.Kubernetes }}'

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
  path: "addons"

apiEndpoint:
  host: '{{ .ApiEndpoint }}'
  port: 6443

controlPlane:
  hosts:
{{- $privateKey := "./private.pem" }}
{{- range $Name, $Value := .Cluster.Ips }}
{{- if eq $Value.IsControl true}}
  - publicAddress: '{{ $Value.Public }}'
    privateAddress: '{{ $Value.Private }}'
    sshPrivateKeyFile: '{{ $privateKey }}'
{{- end}}
{{- end}}

staticWorkers:
  hosts:
{{- range $Name, $Value := .Cluster.Ips }}
{{- if eq $Value.IsControl false}}
  - publicAddress: '{{ $Value.Public }}'
    privateAddress: '{{ $Value.Private }}'
    sshPrivateKeyFile: '{{ $privateKey }}'
{{- end}}
{{- end}}

machineController:
  deploy: false