{{- range $i, $region := .Regions }}
provider "oci" {
  tenancy_ocid      = "{{( index $.NodePools 0).Provider.OciTenancyOcid }}"
  user_ocid         = "{{( index $.NodePools 0).Provider.OciUserOcid }}"
  fingerprint       = "{{( index $.NodePools 0).Provider.OciFingerprint }}"
  private_key_path  = "{{( index $.NodePools 0).Provider.SpecName }}"
  region            = "{{ $region }}"
  alias             = "k8s_nodepool_{{ $region }}"
}
{{- end }}