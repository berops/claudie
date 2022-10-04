{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}
{{$index :=  0}}

provider "aws" {
  region     = "{{(index .NodePools 0).Region}}"
  access_key = "{{(index .NodePools 0).Provider.AccessKey}}"
  secret_key = "{{(index .NodePools 0).Provider.Credentials}}"
}

resource "aws_vpc" "claudie-vpc" {
  cidr_block = "10.0.0.0/16"
  tags = {
    Name = "{{ $clusterName }}-{{ $clusterHash }}-vpc"
  }
}

resource "aws_subnet" "claudie-subnet" {
  vpc_id            = aws_vpc.claudie-vpc.id
  cidr_block        = "10.0.0.0/24"
  map_public_ip_on_launch = true
  tags = {
    Name = "{{ $clusterName }}-{{ $clusterHash }}-subnet"
  }
}

resource "aws_internet_gateway" "claudie-gateway" {
    vpc_id = aws_vpc.claudie-vpc.id
    tags = {
      Name = "{{ $clusterName }}-{{ $clusterHash }}-gateway"
    }
}

resource "aws_route_table" "claudie-route-table" {
    vpc_id = aws_vpc.claudie-vpc.id
    tags = {
      Name = "{{ $clusterName }}-{{ $clusterHash }}-rt"
    }
}

resource "aws_route" "default-route" {
  route_table_id         = aws_route_table.claudie-route-table.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.claudie-gateway.id
}

resource "aws_security_group" "claudie-sg" {
  vpc_id      = aws_vpc.claudie-vpc.id
  tags = {
    Name = "{{ $clusterName }}-{{ $clusterHash }}-sg"
  }
}

resource "aws_security_group_rule" "allow-egress" {
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie-sg.id
}


resource "aws_security_group_rule" "allow-ssh" {
  type              = "ingress"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie-sg.id
}

resource "aws_security_group_rule" "allow-kube-api" {
  type              = "ingress"
  from_port         = 6443
  to_port           = 6443
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie-sg.id
}


resource "aws_security_group_rule" "allow-wireguard" {
  type              = "ingress"
  from_port         = 51820
  to_port           = 51820
  protocol          = "udp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie-sg.id
}

resource "aws_security_group_rule" "allow-icmp" {
  type              = "ingress"
  from_port         = -1
  to_port           = -1
  protocol          = "icmp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.claudie-sg.id
}

resource "aws_key_pair" "claudie-pair" {
  key_name   = "claudie-key"
  public_key = file("./public.pem")
}

{{ range $nodepool := .NodePools }}
resource "aws_instance" "{{ $nodepool.Name }}" {
  count = {{ $nodepool.Count }}
  availability_zone = "{{ $nodepool.Zone }}"
  instance_type = "{{ $nodepool.ServerType }}"
  ami = "{{ $nodepool.Image }}"
  
  associate_public_ip_address = true
  subnet_id = aws_subnet.claudie-subnet.id
  tags = {
    Name = "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-${count.index + 1}"
  }
  key_name = aws_key_pair.claudie-pair.key_name
}

output  "{{ $nodepool.Name }}" {
  value = {
    for node in aws_instance.{{ $nodepool.Name }}:
    node.tags_all.Name => node.public_ip
  }
}
{{end}}