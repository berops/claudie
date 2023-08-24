{{- $clusterName := .ClusterName }}
{{- $clusterHash := .ClusterHash }}

{{- range $i, $region := .Regions }}
resource "aws_vpc" "claudie_vpc_{{ $region }}" {
  provider   = aws.nodepool_{{ $region }}
  cidr_block = "10.0.0.0/16"

  tags = {
    Name            = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-vpc"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_internet_gateway" "claudie_gateway_{{ $region }}" {
  provider = aws.nodepool_{{ $region }}
  vpc_id   = aws_vpc.claudie_vpc_{{ $region }}.id

  tags = {
    Name            = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-gateway"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_route_table" "claudie_route_table_{{ $region }}" {
  provider     = aws.nodepool_{{ $region }}
  vpc_id       = aws_vpc.claudie_vpc_{{ $region }}.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.claudie_gateway_{{ $region }}.id
  }

  tags = {
    Name            = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-rt"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_security_group" "claudie_sg_{{ $region }}" {
  provider               = aws.nodepool_{{ $region }}
  vpc_id                 = aws_vpc.claudie_vpc_{{ $region }}.id
  revoke_rules_on_delete = true

  tags = {
    Name            = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-sg"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_security_group_rule" "allow_egress_{{ $region }}" {
  provider          = aws.nodepool_{{ $region }}
  type              = "egress"
  from_port         = 0
  to_port           = 65535
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}.id
}


resource "aws_security_group_rule" "allow_ssh_{{ $region }}" {
  provider          = aws.nodepool_{{ $region }}
  type              = "ingress"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}.id
}

{{- if eq $.ClusterType "K8s" }}
    {{- if index $.Metadata "loadBalancers" | targetPorts | isMissing 6443 }}
resource "aws_security_group_rule" "allow_kube_api_{{ $region }}" {
  provider          = aws.nodepool_{{ $region }}
  type              = "ingress"
  from_port         = 6443
  to_port           = 6443
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}.id
}
    {{- end }}
{{- end }}


{{- if eq $.ClusterType "LB" }}
    {{- range $role := index $.Metadata "roles" }}
resource "aws_security_group_rule" "allow_{{ $role.Port }}_{{ $region }}" {
  provider          = aws.nodepool_{{ $region }}
  type              = "ingress"
  from_port         = {{ $role.Port }}
  to_port           = {{ $role.Port }}
  protocol          = "{{ $role.Protocol }}"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}.id
}
    {{- end }}
{{- end }}

resource "aws_security_group_rule" "allow_wireguard_{{ $region }}" {
  provider          = aws.nodepool_{{ $region }}
  type              = "ingress"
  from_port         = 51820
  to_port           = 51820
  protocol          = "udp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}.id
}

resource "aws_security_group_rule" "allow_icmp_{{ $region }}" {
  provider          = aws.nodepool_{{ $region }}
  type              = "ingress"
  from_port         = 8
  to_port           = 0
  protocol          = "icmp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}.id
}
{{- end }}

{{- range $i, $nodepool := .NodePools }}
resource "aws_subnet" "{{ $nodepool.Name }}_subnet" {
  provider                = aws.nodepool_{{ $nodepool.NodePool.Region  }}
  vpc_id                  = aws_vpc.claudie_vpc_{{ $nodepool.NodePool.Region }}.id
  cidr_block              = "{{ index $.Metadata (printf "%s-subnet-cidr" $nodepool.Name) }}"
  map_public_ip_on_launch = true
  availability_zone       = "{{ $nodepool.NodePool.Zone }}"

  tags = {
    Name            = "{{ $nodepool.Name }}-{{ $clusterHash }}-subnet"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_route_table_association" "{{ $nodepool.Name }}_rta" {
  provider       = aws.nodepool_{{ $nodepool.NodePool.Region  }}
  subnet_id      = aws_subnet.{{ $nodepool.Name }}_subnet.id
  route_table_id = aws_route_table.claudie_route_table_{{ $nodepool.NodePool.Region }}.id
}
{{- end }}