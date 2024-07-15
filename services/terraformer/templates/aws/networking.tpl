{{- $clusterName := .ClusterData.ClusterName }}
{{- $clusterHash := .ClusterData.ClusterHash }}

{{- range $_, $region := .Regions }}
{{- $specName := $.Provider.SpecName }}

resource "aws_vpc" "claudie_vpc_{{ $region }}_{{ $specName }}" {
  provider   = aws.nodepool_{{ $region }}_{{ $specName }}
  cidr_block = "10.0.0.0/16"

  tags = {
    Name            = "vpc-{{ $clusterHash }}-{{ $region }}-{{ $specName }}"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_internet_gateway" "claudie_gateway_{{ $region }}_{{ $specName }}" {
  provider = aws.nodepool_{{ $region }}_{{ $specName }}
  vpc_id   = aws_vpc.claudie_vpc_{{ $region }}_{{ $specName }}.id

  tags = {
    Name            = "gtw-{{ $clusterHash }}-{{ $region }}-{{ $specName }}"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_route_table" "claudie_route_table_{{ $region }}_{{ $specName }}" {
  provider     = aws.nodepool_{{ $region }}_{{ $specName }}
  vpc_id       = aws_vpc.claudie_vpc_{{ $region }}_{{ $specName }}.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.claudie_gateway_{{ $region }}_{{ $specName }}.id
  }

  tags = {
    Name            = "rt-{{ $clusterHash }}-{{ $region }}-{{ $specName }}"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_security_group" "claudie_sg_{{ $region }}_{{ $specName }}" {
  provider               = aws.nodepool_{{ $region }}_{{ $specName }}
  vpc_id                 = aws_vpc.claudie_vpc_{{ $region }}_{{ $specName }}.id
  revoke_rules_on_delete = true

  tags = {
    Name            = "sg-{{ $clusterHash }}-{{ $region }}-{{ $specName }}"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_security_group_rule" "allow_egress_{{ $region }}_{{ $specName }}" {
  provider          = aws.nodepool_{{ $region }}_{{ $specName }}
  type              = "egress"
  from_port         = 0
  to_port           = 65535
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}_{{ $specName }}.id
}


resource "aws_security_group_rule" "allow_ssh_{{ $region }}_{{ $specName }}" {
  provider          = aws.nodepool_{{ $region }}_{{ $specName }}
  type              = "ingress"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}_{{ $specName }}.id
}

{{- if eq $.ClusterData.ClusterType "K8s" }}
    {{- if index $.Metadata "loadBalancers" | targetPorts | isMissing 6443 }}
resource "aws_security_group_rule" "allow_kube_api_{{ $region }}_{{ $specName }}" {
  provider          = aws.nodepool_{{ $region }}_{{ $specName }}
  type              = "ingress"
  from_port         = 6443
  to_port           = 6443
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}_{{ $specName }}.id
}
    {{- end }}
{{- end }}


{{- if eq $.ClusterData.ClusterType "LB" }}
    {{- range $role := index $.Metadata "roles" }}
resource "aws_security_group_rule" "allow_{{ $role.Port }}_{{ $region }}_{{ $specName }}" {
  provider          = aws.nodepool_{{ $region }}_{{ $specName }}
  type              = "ingress"
  from_port         = {{ $role.Port }}
  to_port           = {{ $role.Port }}
  protocol          = "{{ $role.Protocol }}"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}_{{ $specName }}.id
}
    {{- end }}
{{- end }}

resource "aws_security_group_rule" "allow_wireguard_{{ $region }}_{{ $specName }}" {
  provider          = aws.nodepool_{{ $region }}_{{ $specName }}
  type              = "ingress"
  from_port         = 51820
  to_port           = 51820
  protocol          = "udp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}_{{ $specName }}.id
}

resource "aws_security_group_rule" "allow_icmp_{{ $region }}_{{ $specName }}" {
  provider          = aws.nodepool_{{ $region }}_{{ $specName }}
  type              = "ingress"
  from_port         = 8
  to_port           = 0
  protocol          = "icmp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}_{{ $specName }}.id
}
{{- end }}
