provider "hcloud" {
  token = "{{ (index .NodePools 0).NodePool.Provider.Credentials }}"
  alias = "lb_nodepool"
}