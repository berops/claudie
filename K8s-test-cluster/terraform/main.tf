terraform {
  required_providers {
    google = {
      source = "hashicorp/google"
      version = "3.71.0"
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

resource "google_container_cluster" "primary" {
  name     = var.name
  location = var.zone
  project = var.project
  enable_autopilot = true     # Use autopilot mode -> https://cloud.google.com/kubernetes-engine/docs/concepts/autopilot-overview#comparison
}

