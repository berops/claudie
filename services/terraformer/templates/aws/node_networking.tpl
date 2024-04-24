{{- $clusterName := .ClusterData.ClusterName }}
{{- $clusterHash := .ClusterData.ClusterHash }}

{{- range $i, $nodepool := .NodePools }}
resource "aws_subnet" "{{ $nodepool.Name }}_subnet" {
  provider                = aws.nodepool_{{ $nodepool.NodePool.Region  }}_{{ $nodepool.NodePool.Provider.SpecName}}
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
  provider       = aws.nodepool_{{ $nodepool.NodePool.Region  }}_{{ $nodepool.NodePool.Provider.SpecName}}
  subnet_id      = aws_subnet.{{ $nodepool.Name }}_subnet.id
  route_table_id = aws_route_table.claudie_route_table_{{ $nodepool.NodePool.Region }}.id
}
{{- end }}
