{{- range $i, $region := .Regions }}
provider "oci" {
  tenancy_ocid      = "{{ (index $.NodePools 0).NodePool.Provider.OciTenancyOcid }}"
  user_ocid         = "{{ (index $.NodePools 0).NodePool.Provider.OciUserOcid }}"
  fingerprint       = "{{ (index $.NodePools 0).NodePool.Provider.OciFingerprint }}"
  private_key_path  = "{{ (index $.NodePools 0).NodePool.Provider.SpecName }}"
  region            = "{{ $region }}"
  alias             = "lb_nodepool_{{ $region }}"
}
{{- end }}