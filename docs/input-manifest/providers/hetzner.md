# Hetzner

In Claudie, the Hetzner cloud provider requires you to input the credentials which  contain the API token for your Hetzner project. The API key should have `Read & Write` permissions.

To find out how you can create an API token please follow instructions [here](https://docs.hetzner.com/cloud/api/getting-started/generating-api-token/).

For DNS the instructions for the token can be found [here](https://docs.hetzner.com/dns-console/dns/general/api-access-token/)

## DNS requirements

If your Hetzner provider will be used for DNS, you need to manually

- [set up dns zone](https://www.hetzner.com/dns-console)
- [update domain name server](https://docs.hetzner.com/dns-console/dns/general/dns-overview/#the-hetzner-online-name-servers-are)

since Claudie does not support their dynamic creation.

Note the provider for DNS is different from that for the Cloud.
