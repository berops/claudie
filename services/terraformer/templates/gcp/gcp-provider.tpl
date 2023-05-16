{{- range $i, $region := .Regions}}
provider "google" {
  credentials = "${file("{{ (index $.NodePools 0).Provider.SpecName }}")}"
  project     = "{{ (index $.NodePools 0).Provider.GcpProject }}"
  region      = "{{ $region }}"
  alias       = "k8s_nodepool_{{ $region }}"
}
{{- end}}