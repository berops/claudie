{{- range $_, $region := .Regions }}
provider "genesiscloud" {
    token = "{{ $.Provider.Credentials }}"
    alias = "nodepool_{{ $region }}_{{ $.Provider.SpecName }}"
}
{{- end }}