provider "aws" {
  secret_key = "${file("{{.Provider.SpecName}}")}"
  access_key = "{{.Provider.AwsAccessKey}}"
  # we need to supply some aws region even though the records DNS are global.
  # this a requirement otherwise terraform will exit with an error.
  region     = "eu-central-1"
  alias      = "dns-aws"
}

data "aws_route53_zone" "aws-zone" {
    provider = aws.dns-aws
    name = "{{.DNSZone}}"
}

resource "aws_route53_record" "record" {
    provider = aws.dns-aws
    zone_id = "${data.aws_route53_zone.aws-zone.zone_id}"
    name = "{{ .HostnameHash }}.${data.aws_route53_zone.aws-zone.name}"
    type = "A"
    ttl  = 300
    records = [
    {{range $IP := .NodeIPs}}
    "{{$IP}}",
    {{end}}
    ]
}

output "{{.ClusterName}}-{{.ClusterHash}}" {
    value = { "{{.ClusterName}}-{{.ClusterHash}}-endpoint" = aws_route53_record.record.name }
}