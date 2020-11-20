%{ for index, ip in control_public_ip ~}
${ip}
%{ endfor ~}
%{ for index, ip in compute_public_ip ~}
${ip}
%{ endfor ~}
