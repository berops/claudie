{{- range $i, $region := .Regions}}
provider "google" {
  credentials = "${file("{{ (index $.NodePools 0).NodePool.Provider.SpecName }}")}"
  project     = "{{ (index $.NodePools 0).NodePool.Provider.GcpProject }}"
  region      = "{{ $region }}"
  alias       = "lb_nodepool_{{ $region }}"
}
{{- end}}