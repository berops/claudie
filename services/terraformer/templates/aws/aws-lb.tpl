{{- $clusterName := .ClusterName }}
{{- $clusterHash := .ClusterHash }}
{{- range $i, $region := .Regions }}
resource "aws_vpc" "claudie_vpc_{{ $region }}" {
  provider   = aws.lb_nodepool_{{ $region }}
  cidr_block = "10.0.0.0/16"

  tags = {
    Name            = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-vpc"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_internet_gateway" "claudie_gateway_{{ $region }}" {
  provider = aws.lb_nodepool_{{ $region }}
  vpc_id   = aws_vpc.claudie_vpc_{{ $region }}.id

  tags = {
    Name            = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-gateway"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_route_table" "claudie_route_table_{{ $region }}" {
  provider     = aws.lb_nodepool_{{ $region }}
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
  provider               = aws.lb_nodepool_{{ $region }}
  vpc_id                 = aws_vpc.claudie_vpc_{{ $region }}.id
  revoke_rules_on_delete = true

  tags = {
    Name            = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-sg"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_security_group_rule" "allow_egress_{{ $region }}" {
  provider          = aws.lb_nodepool_{{ $region }}
  type              = "egress"
  from_port         = 0
  to_port           = 65535
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}.id
}


resource "aws_security_group_rule" "allow_ssh_{{ $region }}" {
  provider          = aws.lb_nodepool_{{ $region }}
  type              = "ingress"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}.id
}

resource "aws_security_group_rule" "allow_wireguard_{{ $region }}" {
  provider          = aws.lb_nodepool_{{ $region }}
  type              = "ingress"
  from_port         = 51820
  to_port           = 51820
  protocol          = "udp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}.id
}

{{- range $role := index $.Metadata "roles" }}
resource "aws_security_group_rule" "allow_{{ $role.Port }}_{{ $region }}" {
  provider          = aws.lb_nodepool_{{ $region }}
  type              = "ingress"
  from_port         = {{ $role.Port }}
  to_port           = {{ $role.Port }}
  protocol          = "{{ $role.Protocol }}"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}.id
}
{{- end }}

resource "aws_security_group_rule" "allow_icmp_{{ $region }}" {
  provider          = aws.lb_nodepool_{{ $region }}
  type              = "ingress"
  from_port         = 8
  to_port           = 0
  protocol          = "icmp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie_sg_{{ $region }}.id
}

resource "aws_key_pair" "claudie_pair_{{ $region }}" {
  provider   = aws.lb_nodepool_{{ $region }}
  key_name   = "{{ $clusterName }}-{{ $clusterHash }}-key"
  public_key = file("./public.pem")
  tags = {
    Name            = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-key"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}
{{- end }}

{{- range $i, $nodepool := .NodePools }}
resource "aws_subnet" "{{ $nodepool.Name }}_subnet" {
  provider                = aws.lb_nodepool_{{ $nodepool.Region }}
  vpc_id                  = aws_vpc.claudie_vpc_{{ $nodepool.Region }}.id
  cidr_block              = "{{ index $.Metadata (printf "%s-subnet-cidr" $nodepool.Name) }}"
  map_public_ip_on_launch = true
  availability_zone       = "{{ $nodepool.Zone }}"

  tags = {
    Name            = "{{ $nodepool.Name }}-{{ $clusterHash }}-subnet"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_route_table_association" "{{ $nodepool.Name }}_rta" {
  provider       = aws.lb_nodepool_{{ $nodepool.Region }}
  subnet_id      = aws_subnet.{{ $nodepool.Name }}_subnet.id
  route_table_id = aws_route_table.claudie_route_table_{{ $nodepool.Region }}.id
}


{{- range $node := $nodepool.Nodes }}
resource "aws_instance" "{{ $node.Name }}" {
  provider          = aws.lb_nodepool_{{ $nodepool.Region }}
  availability_zone = "{{ $nodepool.Zone }}"
  instance_type     = "{{ $nodepool.ServerType }}"
  ami               = "{{ $nodepool.Image }}"
  
  associate_public_ip_address = true
  key_name               = aws_key_pair.claudie_pair_{{ $nodepool.Region }}.key_name
  subnet_id              = aws_subnet.{{ $nodepool.Name }}_subnet.id
  vpc_security_group_ids = [aws_security_group.claudie_sg_{{ $nodepool.Region }}.id]

  tags = {
    Name            = "{{ $node.Name }}"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
  
  root_block_device {
    volume_size = {{ $nodepool.DiskSize }}
  }
    # Allow ssh connection for root
    user_data = <<EOF
#!/bin/bash
sed -n 's/^.*ssh-rsa/ssh-rsa/p' /root/.ssh/authorized_keys > /root/.ssh/temp
cat /root/.ssh/temp > /root/.ssh/authorized_keys
rm /root/.ssh/temp
echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && echo "PubkeyAcceptedKeyTypes=+ssh-rsa" >> sshd_config && service sshd restart
EOF
}
{{- end }}

output  "{{ $nodepool.Name }}" {
  value = {
    {{- range $node := $nodepool.Nodes }}
    "${aws_instance.{{ $node.Name }}.tags_all.Name}" =  aws_instance.{{ $node.Name }}.public_ip
    {{- end }}
  }
}
{{- end }}