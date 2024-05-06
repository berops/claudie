provider "hcloud" {
  token = "{{ $.Provider.Credentials }}"
  alias = "nodepool_{{ $.Provider.SpecName }}"
}