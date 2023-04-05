terraform {
  required_providers {
    {{- if .Hetzner }}
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "1.37.0"
    }
    {{- end }}
    {{- if .Gcp }}
    google = {
      source  = "hashicorp/google"
      version = "4.59.0"
    }
    {{- end }}
    {{- if .Aws }}
    aws = {
      source  = "hashicorp/aws"
      version = "4.61.0"
    }
    {{- end }}
    {{- if .Oci }}
    oci = {
      source  = "oracle/oci"
      version = "4.114.0"
    }
    {{- end }}
    {{- if .Azure }}
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "3.50.0"
    }
    {{- end }}
    {{- if .Cloudflare }}
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "4.2.0"
    }
    {{- end }}
    {{- if .HetznerDNS }}
    hetznerdns = {
      source  = "timohirt/hetznerdns"
      version = "2.2.0"
    }
    {{- end }}
  }
}