provider "azurerm" {
  features {}
  subscription_id = "{{.Provider.AzureSubscriptionId}}"
  tenant_id       = "{{.Provider.AzureTenantId}}"
  client_id       = "{{.Provider.AzureClientId}}"
  client_secret   = "${file("{{.Provider.SpecName}}")}"
  alias           = "dns-azure"
}

data "azurerm_dns_zone" "azure-zone" {
    provider = azurerm.dns-azure
    name     = "{{.DNSZone}}"
}

resource "azurerm_dns_a_record" "record" {
  provider            = azurerm.dns-azure
  name                = "{{ .HostnameHash }}"
  zone_name           = data.azurerm_dns_zone.azure-zone.name
  resource_group_name = data.azurerm_dns_zone.azure-zone.resource_group_name
  ttl                 = 300
  records             = [
  {{range $IP := .NodeIPs}}
  "{{$IP}}",
  {{end}}
  ]
}

output "{{.ClusterName}}-{{.ClusterHash}}" {
    value = { "{{.ClusterName}}-{{.ClusterHash}}-endpoint" = format("%s.%s", azurerm_dns_a_record.record.name, azurerm_dns_a_record.record.zone_name)}
}