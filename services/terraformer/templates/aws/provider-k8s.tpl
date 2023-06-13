{{- range $i, $region := .Regions }}
provider "aws" {
  access_key = "{{ (index $.NodePools 0).NodePool.Provider.AwsAccessKey }}"
  secret_key = file("{{ (index $.NodePools 0).NodePool.Provider.SpecName }}")
  region     = "{{ $region }}"
  alias      = "k8s_nodepool_{{ $region }}"
  default_tags {
    tags = {
      Managed-by = "Claudie"
    }
  }
}
{{- end}}