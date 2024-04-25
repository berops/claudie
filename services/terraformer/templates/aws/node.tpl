{{- $clusterName := .ClusterData.ClusterName }}
{{- $clusterHash := .ClusterData.ClusterHash }}

{{- range $_, $nodepool := .NodePools }}

{{- $region   := $nodepool.NodePool.Region }}
{{- $specName := $nodepool.NodePool.Provider.SpecName }}

{{- range $node := $nodepool.Nodes }}
resource "aws_instance" "{{ $node.Name }}_{{ $region }}_{{ $specName }}" {
  provider          = aws.nodepool_{{ $region }}_{{ $specName }}
  availability_zone = "{{ $nodepool.NodePool.Zone }}"
  instance_type     = "{{ $nodepool.NodePool.ServerType }}"
  ami               = "{{ $nodepool.NodePool.Image }}"

  associate_public_ip_address = true
  key_name               = aws_key_pair.claudie_pair_{{ $region }}_{{ $specName }}.key_name
  subnet_id              = aws_subnet.{{ $nodepool.Name }}_{{ $clusterName }}_{{ $clusterHash }}_{{ $region }}_{{ $specName }}_subnet.id
  vpc_security_group_ids = [aws_security_group.claudie_sg_{{ $region }}_{{ $specName }}.id]

  tags = {
    Name            = "{{ $node.Name }}"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }

{{- if eq $.ClusterData.ClusterType "LB" }}
  root_block_device {
    volume_size           = 50
    delete_on_termination = true
    volume_type           = "gp2"
  }

  user_data = <<EOF
#!/bin/bash
# Allow ssh connection for root
sed -n 's/^.*ssh-rsa/ssh-rsa/p' /root/.ssh/authorized_keys > /root/.ssh/temp
cat /root/.ssh/temp > /root/.ssh/authorized_keys
rm /root/.ssh/temp
echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && echo "PubkeyAcceptedKeyTypes=+ssh-rsa" >> sshd_config && service sshd restart
EOF

{{- end }}

{{- if eq $.ClusterData.ClusterType "K8s" }}
  root_block_device {
    volume_size           = 100
    delete_on_termination = true
    volume_type           = "gp2"
  }
  user_data = <<EOF
#!/bin/bash
set -euxo pipefail
# Allow ssh connection for root
sed -n 's/^.*ssh-rsa/ssh-rsa/p' /root/.ssh/authorized_keys > /root/.ssh/temp
cat /root/.ssh/temp > /root/.ssh/authorized_keys
rm /root/.ssh/temp
echo 'PermitRootLogin without-password' >> /etc/ssh/sshd_config && echo 'PubkeyAuthentication yes' >> /etc/ssh/sshd_config && echo "PubkeyAcceptedKeyTypes=+ssh-rsa" >> sshd_config && service sshd restart
# Create longhorn volume directory
mkdir -p /opt/claudie/data
    {{- if and (not $nodepool.IsControl) (gt $nodepool.NodePool.StorageDiskSize 0) }}
# Mount EBS volume only when not mounted yet
sleep 50
disk=$(ls -l /dev/disk/by-id | grep "${replace("${aws_ebs_volume.{{ $node.Name }}_{{ $region }}_{{ $specName }}_volume.id}", "-", "")}" | awk '{print $NF}')
disk=$(basename "$disk")
if ! grep -qs "/dev/$disk" /proc/mounts; then
  if ! blkid /dev/$disk | grep -q "TYPE=\"xfs\""; then
    mkfs.xfs /dev/$disk
  fi
  mount /dev/$disk /opt/claudie/data
  echo "/dev/$disk /opt/claudie/data xfs defaults 0 0" >> /etc/fstab
fi
    {{- end }}
EOF
{{- end }}
}

{{- if eq $.ClusterData.ClusterType "K8s" }}
    {{- if and (not $nodepool.IsControl) (gt $nodepool.NodePool.StorageDiskSize 0) }}
resource "aws_ebs_volume" "{{ $node.Name }}_{{ $region }}_{{ $specName }}_volume" {
  provider          = aws.nodepool_{{ $region }}_{{ $specName }}
  availability_zone = "{{ $nodepool.NodePool.Zone }}"
  size              = {{ $nodepool.NodePool.StorageDiskSize }}
  type              = "gp2"

  tags = {
    Name            = "{{ $node.Name }}d"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_volume_attachment" "{{ $node.Name }}_{{ $region }}_{{ $specName }}_volume_att" {
  provider    = aws.nodepool_{{ $region }}_{{ $specName }}
  device_name = "/dev/sdh"
  volume_id   = aws_ebs_volume.{{ $node.Name }}_{{ $region }}_{{ $specName }}_volume.id
  instance_id = aws_instance.{{ $node.Name }}_{{ $region }}_{{ $specName }}.id
}
    {{- end }}
{{- end }}

{{- end }}

output  "{{ $nodepool.Name }}" {
  value = {
    {{- range $_, $node := $nodepool.Nodes }}
    "${aws_instance.{{ $node.Name }}_{{ $region }}_{{ $specName }}.tags_all.Name}" =  aws_instance.{{ $node.Name }}_{{ $region }}_{{ $specName }}.public_ip
    {{- end }}
  }
}
{{- end }}
