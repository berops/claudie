%{ for index, control in control ~}
${control.ipv4_address}
${control.name}
%{ endfor ~}
%{ for index, compute in compute ~}
${compute.ipv4_address}
${compute.name}
%{ endfor ~}
