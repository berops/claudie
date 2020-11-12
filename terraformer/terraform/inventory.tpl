[nodes]
%{ for index, ip in public_ip ~}
node_${index+1} ansible_host=${ip} private_ip=192.168.2.${index+1}
%{ endfor ~}