%{ for index, control in control ~}
${control.network_interface.0.access_config.0.nat_ip}
${control.name}
%{ endfor ~}
%{ for index, compute in compute ~}
${compute.network_interface.0.access_config.0.nat_ip}
${compute.name}
%{ endfor ~}
