{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}
{{$index :=  0}}

{{- range $i, $region := .Regions }}
provider "aws" {
  access_key = "{{(index $.NodePools 0).Provider.AwsAccessKey}}"
  secret_key = file("{{(index $.NodePools 0).Provider.SpecName}}")
  region     = "{{ $region }}"
  alias      = "k8s-nodepool-{{ $region }}"
}

resource "aws_vpc" "claudie-vpc-{{ $region }}" {
  provider   = aws.k8s-nodepool-{{ $region }}
  cidr_block = "10.0.0.0/16"
  tags       = {
    Name     = "{{ $clusterName }}-{{ $clusterHash }}-vpc"
  }
}

resource "aws_internet_gateway" "claudie-gateway-{{ $region }}" {
  provider = aws.k8s-nodepool-{{ $region }}
  vpc_id   = aws_vpc.claudie-vpc-{{ $region }}.id
  tags     = {
    Name   = "{{ $clusterName }}-{{ $clusterHash }}-gateway"
  }
}

resource "aws_route_table" "claudie-route-table-{{ $region }}" {
  provider     = aws.k8s-nodepool-{{ $region }}
  vpc_id       = aws_vpc.claudie-vpc-{{ $region }}.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.claudie-gateway-{{ $region }}.id
  }
  tags         = {
    Name       = "{{ $clusterName }}-{{ $clusterHash }}-rt"
  }
}

resource "aws_security_group" "claudie-sg-{{ $region }}" {
  provider               = aws.k8s-nodepool-{{ $region }}
  vpc_id                 = aws_vpc.claudie-vpc-{{ $region }}.id
  revoke_rules_on_delete = true
  tags                   = {
    Name                 = "{{ $clusterName }}-{{ $clusterHash }}-sg"
  }
}

resource "aws_security_group_rule" "allow-egress-{{ $region }}" {
  provider          = aws.k8s-nodepool-{{ $region }}
  type              = "egress"
  from_port         = 0
  to_port           = 65535
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie-sg-{{ $region }}.id
}


resource "aws_security_group_rule" "allow-ssh-{{ $region }}" {
  provider          = aws.k8s-nodepool-{{ $region }}
  type              = "ingress"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie-sg-{{ $region }}.id
}
{{- if index $.Metadata "loadBalancers" | targetPorts | isMissing 6443 }}
resource "aws_security_group_rule" "allow-kube-api-{{ $region }}" {
  provider          = aws.k8s-nodepool-{{ $region }}
  type              = "ingress"
  from_port         = 6443
  to_port           = 6443
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie-sg-{{ $region }}.id
}
{{- end}}

resource "aws_security_group_rule" "allow-wireguard-{{ $region }}" {
  provider          = aws.k8s-nodepool-{{ $region }}
  type              = "ingress"
  from_port         = 51820
  to_port           = 51820
  protocol          = "udp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie-sg-{{ $region }}.id
}

resource "aws_security_group_rule" "allow-icmp-{{ $region }}" {
  provider          = aws.k8s-nodepool-{{ $region }}
  type              = "ingress"
  from_port         = 8
  to_port           = 0
  protocol          = "icmp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie-sg-{{ $region }}.id
}

resource "aws_key_pair" "claudie-pair-{{ $region }}" {
  provider   = aws.k8s-nodepool-{{ $region }}
  key_name   = "{{ $clusterName }}-{{ $clusterHash }}-key"
  public_key = file("./public.pem")
}
{{- end }}

{{- range $i, $nodepool := .NodePools }}
resource "aws_subnet" "{{ $nodepool.Name }}-subnet" {
  provider                = aws.k8s-nodepool-{{ $nodepool.Region  }}
  vpc_id                  = aws_vpc.claudie-vpc-{{ $nodepool.Region }}.id
  cidr_block              = "{{ getCIDR "10.0.0.0/24" 2 $i}}"
  map_public_ip_on_launch = true
  availability_zone       = "{{ $nodepool.Zone }}"
  tags                    = {
    Name                  = "{{ $nodepool.Name }}-{{ $clusterHash }}-subnet"
  }
}

resource "aws_route_table_association" "{{ $nodepool.Name }}-rta" {
  provider       = aws.k8s-nodepool-{{ $nodepool.Region  }}
  subnet_id      = aws_subnet.{{ $nodepool.Name }}-subnet.id
  route_table_id = aws_route_table.claudie-route-table-{{ $nodepool.Region }}.id
}

resource "aws_instance" "{{ $nodepool.Name }}" {
  provider          = aws.k8s-nodepool-{{ $nodepool.Region  }}
  count             = {{ $nodepool.Count }}
  availability_zone = "{{ $nodepool.Zone }}"
  instance_type     = "{{ $nodepool.ServerType }}"
  ami               = "{{ $nodepool.Image }}"
  
  associate_public_ip_address = true
  key_name               = aws_key_pair.claudie-pair-{{ $nodepool.Region }}.key_name
  subnet_id              = aws_subnet.{{ $nodepool.Name }}-subnet.id
  vpc_security_group_ids = [aws_security_group.claudie-sg-{{ $nodepool.Region }}.id]

  tags                   = {
    Name                 = "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-${count.index + 1}"
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

output  "{{ $nodepool.Name }}" {
  value = {
    for node in aws_instance.{{ $nodepool.Name }}:
    node.tags_all.Name => node.public_ip
  }
}
{{end}}