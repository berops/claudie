terraform {
  required_providers {
    {{- if .Hetzner }}
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.60.0"
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
      version = "3.107.0"
    }
    {{- end }}
    {{- if .Cloudflare }}
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "4.52.1"
    }
    {{- end }}
    {{- if .HetznerDNS }}
    hetznerdns = {
      source  = "timohirt/hetznerdns"
      version = "2.2.0"
    }
    {{- end }}
    {{- if .Openstack }}
    openstack = {
        source = "terraform-provider-openstack/openstack"
        version = "3.3.2"
    }
    {{- end }}
    {{- if .Exoscale }}
    exoscale = {
      source  = "exoscale/exoscale"
      version = "~> 0.68"
    }
    {{- end }}
  }
}
