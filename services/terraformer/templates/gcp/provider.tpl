{{- range $i, $region := .Regions}}
provider "google" {
  credentials = "${file("{{ $.Provider.SpecName }}")}"
  project     = "{{ $.Provider.GcpProject }}"
  region      = "{{ $region }}"
  alias       = "nodepool_{{ $region }}_{{ $.Provider.SpecName }}"
}
{{- end}}