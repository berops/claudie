terraform {
  required_providers {
    google = {
      source = "hashicorp/google"
      version = "4.11.0"
    }
  }
}

provider "google" {
    credentials = "${file("../../../../../keys/platform-infrastructure-316112-bd7953f712df.json")}"
    region = "europe-west1"
    project = "platform-infrastructure-316112"
}

resource "google_dns_managed_zone" "{{ .LBName}}" {
  name        = "{{.ClusterName}}-{{.ClusterHash}}-{{ .LBName }}"
  dns_name    = "{{ .Hostname}}"
  visibility = "public"
}

{{- $lbName := .LBName}}
{{- range $nodepool := .NodePools}}
resource "google_dns_record_set" "{{$nodepool.Name}}-{{$lbName}}" {

  name = "{{ .SubDomain }}.${data.google_dns_managed_zone.lb-zone.dns_name}"
  type = "A"
  ttl  = 300

  managed_zone = data.google_dns_managed_zone.lb-zone.name

  rrdatas = [
      for node in google_compute_instance.{{$nodepool.Name}} :node.ipv4_address
    ]
}
{{- end}}