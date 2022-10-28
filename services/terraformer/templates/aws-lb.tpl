{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}
{{$index :=  0}}

provider "aws" {
  region     = "{{(index .NodePools 0).Region}}"
  access_key = "{{(index .NodePools 0).Provider.AwsAccessKey}}"
  secret_key = file("{{(index .NodePools 0).Provider.SpecName}}")
  alias = "lb-nodepool"
}

resource "aws_vpc" "claudie-vpc" {
  provider = aws.lb-nodepool
  cidr_block = "10.0.0.0/16"
  tags = {
    Name = "{{ $clusterName }}-{{ $clusterHash }}-vpc"
  }
}

resource "aws_subnet" "claudie-subnet" {
  provider = aws.lb-nodepool
  vpc_id            = aws_vpc.claudie-vpc.id
  cidr_block        = "10.0.0.0/24"
  map_public_ip_on_launch = true
  availability_zone = "{{(index .NodePools 0).Zone}}"
  tags = {
    Name = "{{ $clusterName }}-{{ $clusterHash }}-subnet"
  }
}

resource "aws_internet_gateway" "claudie-gateway" {
  provider = aws.lb-nodepool
  vpc_id = aws_vpc.claudie-vpc.id
  tags = {
    Name = "{{ $clusterName }}-{{ $clusterHash }}-gateway"
  }
}

resource "aws_route_table" "claudie-route-table" {
  provider = aws.lb-nodepool
  vpc_id = aws_vpc.claudie-vpc.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.claudie-gateway.id
  }
  tags = {
    Name = "{{ $clusterName }}-{{ $clusterHash }}-rt"
  }
}

resource "aws_route_table_association" "claudie-rta" {
  provider = aws.lb-nodepool
  subnet_id = aws_subnet.claudie-subnet.id
  route_table_id = aws_route_table.claudie-route-table.id
}

resource "aws_security_group" "claudie-sg" {
  provider = aws.lb-nodepool
  vpc_id      = aws_vpc.claudie-vpc.id
  revoke_rules_on_delete = true
  tags = {
    Name = "{{ $clusterName }}-{{ $clusterHash }}-sg"
  }
}

resource "aws_security_group_rule" "allow-egress" {
  provider = aws.lb-nodepool
  type              = "egress"
  from_port         = 0
  to_port           = 65535
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie-sg.id
}


resource "aws_security_group_rule" "allow-ssh" {
  provider = aws.lb-nodepool
  type              = "ingress"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie-sg.id
}

resource "aws_security_group_rule" "allow-wireguard" {
  provider = aws.lb-nodepool
  type              = "ingress"
  from_port         = 51820
  to_port           = 51820
  protocol          = "udp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie-sg.id
}

{{range $role := index .Metadata "roles"}}
resource "aws_security_group_rule" "allow-{{ $role.Port }}" {
  provider = aws.lb-nodepool
  type              = "ingress"
  from_port         = {{ $role.Port }}
  to_port           = {{ $role.Port }}
  protocol          = "{{ $role.Protocol }}"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie-sg.id
}
{{end}}

resource "aws_security_group_rule" "allow-icmp" {
  provider = aws.lb-nodepool
  type              = "ingress"
  from_port = 8
  to_port = 0
  protocol = "icmp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie-sg.id
}

resource "aws_key_pair" "claudie-pair" {
  provider = aws.lb-nodepool
  key_name   = "{{ $clusterName }}-{{ $clusterHash }}-key"
  public_key = file("./public.pem")
}

{{ range $nodepool := .NodePools }}
resource "aws_instance" "{{ $nodepool.Name }}" {
  provider = aws.lb-nodepool
  count = {{ $nodepool.Count }}
  availability_zone = "{{ $nodepool.Zone }}"
  instance_type = "{{ $nodepool.ServerType }}"
  ami = "{{ $nodepool.Image }}"
  
  associate_public_ip_address = true
  key_name = aws_key_pair.claudie-pair.key_name
  subnet_id = aws_subnet.claudie-subnet.id
  vpc_security_group_ids = [aws_security_group.claudie-sg.id]

  tags = {
    Name = "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-${count.index + 1}"
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