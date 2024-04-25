{{- range $_, $region := .Regions }}
provider "aws" {
  access_key = "{{ $.Provider.AwsAccessKey }}"
  secret_key = file("{{ $.Provider.SpecName }}")
  region     = "{{ $region }}"
  alias      = "nodepool_{{ $region }}_{{ $.Provider.SpecName }}"
  default_tags {
    tags = {
      Managed-by = "Claudie"
    }
  }
}
{{- end}}
