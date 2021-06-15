variable "project" {
  type = string
}

variable "credentials_file" { }

variable "region" {
  default = "eu"    # Avialable regions: us, eu, asia
}

variable "zone" {
  default = "europe-west3-a"
}