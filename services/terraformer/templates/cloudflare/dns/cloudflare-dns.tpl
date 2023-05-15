provider "cloudflare" {
  api_token = "${file("{{ .Provider.SpecName }}")}"
  alias = "cloudflare_dns"
}

data "cloudflare_zone" "cloudflare-zone" {
  provider   = cloudflare.cloudflare_dns
  name       = "{{ .DNSZone }}"
}

{{- range $IP := .NodeIPs }}
resource "cloudflare_record" "record-{{ replaceAll $IP "." "-" }}" {
  provider = cloudflare.cloudflare_dns
  zone_id  = data.cloudflare_zone.cloudflare-zone.id
  name     = "{{ $.HostnameHash }}"
  value    = "{{ $IP }}"
  type     = "A"
  ttl      = 300
}
{{- end }}

output "{{ .ClusterName }}-{{ .ClusterHash }}" {
  value = { "{{ .ClusterName }}-{{ .ClusterHash }}-endpoint" = format("%s.%s", "{{ .HostnameHash }}", "{{ .DNSZone }}")}
}