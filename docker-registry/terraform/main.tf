terraform {
  required_providers {
    google = {
      source = "hashicorp/google"
      version = "3.70.0"
    }
  }
}
# set provider
provider "google" {
  credentials = file(var.credentials_file)
  project = var.project
  region  = var.region
  zone    = var.zone
}
# Check if bucket exists, if not, create one. Cannot delete the bucket - MUST be deleted manually
resource "google_container_registry" "docker-registry" {
  project  = var.project
  location = var.region
}
