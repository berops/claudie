provider "hetznerdns" {
    apitoken = "${file("{{.Provider.SpecName}}")}"
    alias = "hetzner-dns"
}

data "hetznerdns_zone" "hetzner-zone" {
  provider = hetznerdns.hetzner-dns
    name = "{{.DNSZone}}"
}

{{range $IP := .NodeIPs}}
resource "hetznerdns_record" "record-{{replaceAll $IP "." "-"}}" {
  provider = hetznerdns.hetzner-dns
  zone_id = data.hetznerdns_zone.hetzner-zone.id
  name = "{{ $.HostnameHash }}"
  value = "{{$IP}}"
  type = "A"
  ttl= 300
}
{{end}}

output "{{.ClusterName}}-{{.ClusterHash}}" {
  value = { "{{.ClusterName}}-{{.ClusterHash}}-endpoint" = format("%s.%s", "{{ .HostnameHash }}", "{{.DNSZone}}")}
}