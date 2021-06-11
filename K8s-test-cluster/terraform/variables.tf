variable "project" {
  type = string
}

variable "credentials_file" { }

variable "region" {
  default = "eu"  
}

variable "zone" {
  default = "europe-west3"
}

variable "name" {
    default = "k8s-test-cluster"
}

