{{- $clusterName := .ClusterData.ClusterName }}
{{- $clusterHash := .ClusterData.ClusterHash }}

{{- range $i, $nodepool := .NodePools }}
{{- $region   := $nodepool.NodePool.Region }}
{{- $specName := $nodepool.NodePool.Provider.SpecName }}
resource "aws_subnet" "{{ $nodepool.Name }}_{{ $clusterName }}_{{ $clusterHash }}_{{ $region }}_{{ $specName }}_subnet" {
  provider                = aws.nodepool_{{ $region  }}_{{ $specName}}
  vpc_id                  = aws_vpc.claudie_vpc_{{ $region }}_{{ $specName}}.id
  cidr_block              = "{{ index $.Metadata (printf "%s-subnet-cidr" $nodepool.Name) }}"
  map_public_ip_on_launch = true
  availability_zone       = "{{ $nodepool.NodePool.Zone }}"

  tags = {
    Name            = "snt-{{ $clusterHash }}-{{ $region }}-{{ $nodepool.Name }}"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_route_table_association" "{{ $nodepool.Name }}_{{ $clusterName }}_{{ $clusterHash }}_{{ $region }}_{{ $specName }}_rta" {
  provider       = aws.nodepool_{{ $region  }}_{{ $specName}}
  subnet_id      = aws_subnet.{{ $nodepool.Name }}_{{ $clusterName }}_{{ $clusterHash }}_{{ $region }}_{{ $specName }}_subnet.id
  route_table_id = aws_route_table.claudie_route_table_{{ $region }}_{{ $specName }}.id
}
{{- end }}
