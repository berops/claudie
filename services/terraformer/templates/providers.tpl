terraform {
  required_providers {
    {{ if .Hetzner }}
    hcloud = {
      source = "hetznercloud/hcloud"
      version = "1.35.1"
    }
    {{ end }}
    {{ if .Gcp }}
    google = {
      source = "hashicorp/google"
      version = "4.31.0"
    }
    {{ end }}
    {{if .Aws }}
    aws = {
      source = "hashicorp/aws"
      version = "4.31.0"
    }
    {{ end }}
  }
}