provider "azurerm" {
  features {}
  subscription_id = "{{ (index $.NodePools 0).Provider.AzureSubscriptionId }}"
  tenant_id       = "{{ (index $.NodePools 0).Provider.AzureTenantId }}"
  client_id       = "{{ (index $.NodePools 0).Provider.AzureClientId }}"
  client_secret   = file("{{ (index $.NodePools 0).Provider.SpecName }}")
  alias           = "k8s_nodepool"
}