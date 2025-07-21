terraform {
  required_providers {
    {{- if .Hetzner }}
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "1.51.0"
    }
    {{- end }}
    {{- if .Gcp }}
    google = {
      source  = "hashicorp/google"
      version = "6.44.0"
    }
    {{- end }}
    {{- if .Aws }}
    aws = {
      source  = "hashicorp/aws"
      version = "6.4.0"
    }
    {{- end }}
    {{- if .Oci }}
    oci = {
      source  = "oracle/oci"
      version = "7.10.0"
    }
    {{- end }}
    {{- if .Azure }}
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "4.37.0"
    }
    {{- end }}
    {{- if .Cloudflare }}
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "5.7.1"
    }
    {{- end }}
    {{- if .HetznerDNS }}
    hetznerdns = {
      source  = "timohirt/hetznerdns"
      version = "2.2.0"
    }
    {{- end }}
    {{- if .GenesisCloud }}
    genesiscloud = {
        source = "genesiscloud/genesiscloud"
        version = "1.1.14"
    }
    {{- end }}
  }
}
