apiVersion: kubeone.k8c.io/v1beta2
kind: KubeOneCluster
name: '{{ .ClusterName }}'

versions:
  kubernetes: '{{ .KubernetesVersion }}'

features:
  coreDNS:
    replicas: 2
    deployPodDisruptionBudget: true

clusterNetwork:
  cni:
    cilium:
      enableHubble: true

cloudProvider:
  none: {}
  external: false

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
    {{- if $nodepool.IsDynamic }}
    sshUsername: root
    sshPrivateKeyFile: '{{ $privateKey }}'
    {{- else }}
    sshUsername: '{{ $nodeInfo.Node.Username }}'
    sshPrivateKeyFile: './{{ $nodeInfo.Name }}.pem'
    {{- end }}
    hostname: '{{ $nodeInfo.Name }}'
    {{- if eq $nodeInfo.Node.Public $.APIEndpoint }}
    isLeader: true
    {{- end }}
    taints:
    - key: "node-role.kubernetes.io/control-plane"
      effect: "NoSchedule"
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
    {{- if $nodepool.IsDynamic }}
    sshUsername: root
    sshPrivateKeyFile: '{{ $privateKey }}'
    {{- else }}
    sshUsername: '{{ $nodeInfo.Node.Username }}'
    sshPrivateKeyFile: './{{ $nodeInfo.Name }}.pem'
    {{- end }}
    hostname: '{{ $nodeInfo.Name }}'
    {{- end}}
  {{- end}}
{{- end}}

machineController:
  deploy: false
