{{- range $_, $region := .Regions }}
{{- $sanitisedRegion := replaceAll $region " " "_"}}
provider "azurerm" {
  features {}
  subscription_id = "{{ $.Provider.AzureSubscriptionId }}"
  tenant_id       = "{{ $.Provider.AzureTenantId }}"
  client_id       = "{{ $.Provider.AzureClientId }}"
  client_secret   = file("{{ $.Provider.SpecName }}")
  alias           = "nodepool_{{ $sanitisedRegion }}_{{ $.Provider.SpecName }}"
}
{{- end}}
