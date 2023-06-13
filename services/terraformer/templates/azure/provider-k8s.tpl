provider "azurerm" {
  features {}
  subscription_id = "{{ (index $.NodePools 0).NodePool.Provider.AzureSubscriptionId }}"
  tenant_id       = "{{ (index $.NodePools 0).NodePool.Provider.AzureTenantId }}"
  client_id       = "{{ (index $.NodePools 0).NodePool.Provider.AzureClientId }}"
  client_secret   = file("{{ (index $.NodePools 0).NodePool.Provider.SpecName }}")
  alias           = "k8s_nodepool"
}