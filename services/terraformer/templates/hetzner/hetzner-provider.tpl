provider "hcloud" {
  token = "{{ (index .NodePools 0).Provider.Credentials }}"
  alias = "k8s_nodepool"
}