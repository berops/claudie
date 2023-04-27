{{- range $i, $region := .Regions }}
provider "aws" {
  access_key = "{{ (index $.NodePools 0).Provider.AwsAccessKey }}"
  secret_key = file("{{ (index $.NodePools 0).Provider.SpecName }}")
  region     = "{{ $region }}"
  alias      = "lb_nodepool_{{ $region }}"
  default_tags {
    tags = {
      Managed-by = "Claudie"
    }
  }
}
{{- end}}