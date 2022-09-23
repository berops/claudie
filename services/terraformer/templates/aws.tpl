{{- $clusterName := .ClusterName}}
{{- $clusterHash := .ClusterHash}}
{{$index :=  0}}

provider "aws" {
  region     = "{{(index .NodePools 0).Region}}"
  access_key = "{{(index .NodePools 0).Provider.AccessKey}}"
  secret_key = "{{(index .NodePools 0).Provider.Credentials}}"
}

resource "aws_vpc" "claudie_vpc" {
  cidr_block = "10.0.3.0/16"
  tags = {
    Name = "{{ $clusterName }}-{{ $clusterHash }}-vpc"
  }
}

resource "aws_gateway" "claudie_gateway" {
    vpc_id = aws_vpc.claudie_vpc.id
    tags = {
      Name = "{{ $clusterName }}-{{ $clusterHash }}-gateway"
    }
}

resource "aws_route_table" "claudie_route_table" {
    vpc_id = aws_vpc.claudie_vpc.id
    tags = {
      Name = "{{ $clusterName }}-{{ $clusterHash }}-rt"
    }
}

resource "aws_route" "default_route" {
  route_table_id         = aws_route_table.claudie_route_table.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.claudie_gateway.id
}

resource "aws_security_group" "claudie_sg" {
  for_each    = var.security_groups
  name        = each.value.name
  description = each.value.description
  vpc_id      = aws_vpc.vpc.id

  dynamic "ingress" {
    for_each = each.value.ingress

    content {
      from_port   = ingress.value.from
      to_port     = ingress.value.to
      protocol    = ingress.value.protocol
      cidr_blocks = ingress.value.cidr_blocks
    }
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = -1
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_key_pair" "claudie_pair" {
  key_name   = "claudie-key"
  public_key = file("./public.pem")
}

{{ range $nodepool := .NodePools }}
resource "aws_subnet" "{{ $nodepool.Name }}_subnet" {
  vpc_id            = aws_vpc.claudie_vpc.id
  cidr_block        = "10.0.3.0/8"
  availability_zone = "{{ $nodepool.Zone }}"
  map_public_ip_on_launch = true
  tags = {
    Name = "{{ $clusterName }}-{{ $clusterHash }}-subnet"
  }
}

resource "aws_instance" "{{ $nodepool.Name }}" {
  count = {{ $nodepool.Count }}
  availability_zone = "{{ $nodepool.Zone }}"
  instance_type = "{{ $nodepool.ServerType }}"
  ami = "{{ $nodepool.Image }}"
  
  associate_public_ip_address = true
  subnet_id = aws_subnet.{{ $nodepool.Name }}_subnet.id
  tags {
    Name = "{{ $clusterName }}-{{ $clusterHash }}-{{ $nodepool.Name }}-${count.index + 1}"
  }
  key_name = aws_key_pair.claudie_key.key_name
}

output  "{{ $nodepool.Name }}" {
  value = {
    for node in aws_instance.{{ $nodepool.Name }}:
    node.name => node.public_ip
  }
}
{{end}}