provider "hcloud" {
  token = "{{ (index .NodePools 0).Provider.Credentials }}"
  alias = "lb_nodepool"
}