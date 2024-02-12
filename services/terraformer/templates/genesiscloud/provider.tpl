{{- range $i, $region := .Regions }}
provider "genesiscloud" {
    token = "{{ (index $.NodePools 0).NodePool.Provider.Credentials }}"
    alias = "nodepool_{{ $region }}"
}
{{- end }}