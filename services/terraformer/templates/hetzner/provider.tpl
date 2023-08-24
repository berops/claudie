provider "hcloud" {
  token = "{{ (index .NodePools 0).NodePool.Provider.Credentials }}"
  alias = "nodepool"
}