{{- range $i, $region := .Regions}}
provider "google" {
  credentials = "${file("{{ (index $.NodePools 0).Provider.SpecName }}")}"
  project     = "{{ (index $.NodePools 0).Provider.GcpProject }}"
  region      = "{{ $region }}"
  alias       = "lb_nodepool_{{ $region }}"
}
{{- end}}