provider "oci" {
  tenancy_ocid = "{{.Provider.OciTenancyOcid}}"
  user_ocid = "{{.Provider.OciUserOcid}}"
  fingerprint = "{{.Provider.OciFingerprint}}"
  private_key_path = "{{.Provider.SpecName}}"
  region = "eu-frankfurt-1"
  alias = "dns-oci"
}

data "oci_dns_zones" "oci-zone" {
    provider = oci.dns-oci
    compartment_id = "{{.Provider.OciCompartmentOcid}}"
    name = "{{.DNSZone}}"
}

resource "oci_dns_rrset" "record" {
    provider = oci.dns-oci
    domain = "{{ .HostnameHash }}.${data.oci_dns_zones.oci-zone.name}"
    rtype = "A"
    zone_name_or_id = data.oci_dns_zones.oci-zone.name

    compartment_id = "{{.Provider.OciCompartmentOcid}}"
    {{range $IP := .NodeIPs}}
    items {
       domain = "{{ $.HostnameHash }}.${data.oci_dns_zones.oci-zone.name}"
       rdata = "{{$IP}}"
       rtype = "A"
       ttl = 300
    }
    {{end}}
}

output "{{.ClusterName}}-{{.ClusterHash}}" {
  value = { "{{.ClusterName}}-{{.ClusterHash}}-endpoint" = oci_dns_rrset.record.domain }
}