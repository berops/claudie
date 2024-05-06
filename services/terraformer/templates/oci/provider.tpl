{{- range $i, $region := .Regions }}
provider "oci" {
  tenancy_ocid      = "{{ $.Provider.OciTenancyOcid }}"
  user_ocid         = "{{ $.Provider.OciUserOcid }}"
  fingerprint       = "{{ $.Provider.OciFingerprint }}"
  private_key_path  = "{{ $.Provider.SpecName }}"
  region            = "{{ $region }}"
  alias             = "nodepool_{{ $region }}_{{ $.Provider.SpecName }}"
}
{{- end }}