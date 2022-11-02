terraform {
  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "1.35.1"
    }
    google = {
      source  = "hashicorp/google"
      version = "4.31.0"
    }
    aws = {
      source  = "hashicorp/aws"
      version = "4.31.0"
    }
    oci = {
      source  = "oracle/oci"
      version = "4.94.0"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "3.26.0"
    }
  }
}
