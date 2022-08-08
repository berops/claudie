terraform {
  required_providers {
    hcloud = {
      source = "hetznercloud/hcloud"
      version = "1.35.1"
    }
    google = {
      source = "hashicorp/google"
      version = "4.31.0"
    }
  }
}