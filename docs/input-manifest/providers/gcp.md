# GCP

In Claudie, GCP cloud provider requires you to input `credentials` as well as specific project where the account resides. The credentials are in form of an account key in JSON. It is important, that account has sufficient IAM roles attached to it, so Claudie can create all resources for your infrastructure.

Furthermore, your project should enable a few API, namely

- `Compute Engine API`
- `Cloud DNS API`

> when project will be used for Loadbalancers DNS

## DNS requirements

If your GCP provider will be used for DNS, you need to manually

- [set up dns zone](https://cloud.google.com/dns/docs/zones)
- [update domain name server](https://cloud.google.com/dns/docs/update-name-servers)

since Claudie does not support their dynamic creation.

## IAM policies required by Claudie

- for infrastructure creation
  - `roles/roles/compute.admin`
  
- for DNS
  - `roles/dns.admin`
