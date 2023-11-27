{{- $clusterName := .ClusterName }}
{{- $clusterHash := .ClusterHash }}

{{- range $i, $region := .Regions }}
resource "aws_key_pair" "claudie_pair_{{ $region }}" {
  provider   = aws.nodepool_{{ $region }}
  key_name   = "{{ $clusterName }}-{{ $clusterHash }}-key"
  public_key = file("./public.pem")
  tags = {
    Name            = "{{ $clusterName }}-{{ $clusterHash }}-{{ $region }}-key"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}
{{- end }}

{{- range $i, $nodepool := .NodePools }}
{{- range $node := $nodepool.Nodes }}
resource "aws_instance" "{{ $node.Name }}" {
  provider          = aws.nodepool_{{ $nodepool.NodePool.Region }}
  availability_zone = "{{ $nodepool.NodePool.Zone }}"
  instance_type     = "{{ $nodepool.NodePool.ServerType }}"
  ami               = "{{ $nodepool.NodePool.Image }}"

  associate_public_ip_address = true
  key_name               = aws_key_pair.claudie_pair_{{ $nodepool.NodePool.Region }}.key_name
  subnet_id              = aws_subnet.{{ $nodepool.Name }}_subnet.id
  vpc_security_group_ids = [aws_security_group.claudie_sg_{{ $nodepool.NodePool.Region }}.id]

  tags = {
    Name            = "{{ $node.Name }}"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }

{{- if eq $.ClusterType "LB" }}
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

{{- if eq $.ClusterType "K8s" }}
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
    {{- if not $nodepool.IsControl }}
# Mount EBS volume only when not mounted yet
sleep 50
disk=$(ls -l /dev/disk/by-id | grep "${replace("${aws_ebs_volume.{{ $node.Name }}_volume.id}", "-", "")}" | awk '{print $NF}')
disk=$(basename "$disk")
if ! grep -qs "/dev/$disk" /proc/mounts; then
  mkdir -p /opt/claudie/data
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

{{- if eq $.ClusterType "K8s" }}
    {{- if not $nodepool.IsControl and $nodepool.NodePool.StorageDiskSize > 0 }}
resource "aws_ebs_volume" "{{ $node.Name }}_volume" {
  provider          = aws.nodepool_{{ $nodepool.NodePool.Region }}
  availability_zone = "{{ $nodepool.NodePool.Zone }}"
  size              = {{ $nodepool.NodePool.StorageDiskSize }}
  type              = "gp2"

  tags = {
    Name            = "{{ $node.Name }}-storage"
    Claudie-cluster = "{{ $clusterName }}-{{ $clusterHash }}"
  }
}

resource "aws_volume_attachment" "{{ $node.Name }}_volume_att" {
  provider    = aws.nodepool_{{ $nodepool.NodePool.Region }}
  device_name = "/dev/sdh"
  volume_id   = aws_ebs_volume.{{ $node.Name }}_volume.id
  instance_id = aws_instance.{{ $node.Name }}.id
}
    {{- end }}
{{- end }}

{{- end }}

output  "{{ $nodepool.Name }}" {
  value = {
    {{- range $j, $node := $nodepool.Nodes }}
    {{- $name := (printf "%s-%s-%s-%d" $clusterName $clusterHash $nodepool.Name $j ) }}
    "${aws_instance.{{ $node.Name }}.tags_all.Name}" =  aws_instance.{{ $node.Name }}.public_ip
    {{- end }}
  }
}
{{- end }}