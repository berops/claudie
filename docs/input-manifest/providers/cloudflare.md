# Cloudflare
Cloudflare provider requires `apitoken` token field in string format.

## DNS example
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cloudflare-secret
data:
  apitoken: a3NsSVNBODc4YTZldFlBZlhZY2c1aVl5ckZHTmxDeGM=
type: Opaque
```

## Create Cloudflare credentials
You can create Cloudflare API token by following [this guide](https://developers.cloudflare.com/fundamentals/api/get-started/create-token/). The required permissions for the zone you want to use are:

```bash
Zone:Read
DNS:Read
DNS:Edit
```

## DNS setup
If you wish to use Cloudflare as your DNS provider where Claudie creates DNS records pointing to Claudie managed clusters, you will need to create a **public DNS zone** by following [this guide](https://developers.cloudflare.com/dns/zone-setups/).

!!! warning "Cloudflare is not my domain registrar"
    If you haven't acquired a domain via Cloudflare and wish to utilize Cloudflare for hosting your zone, you can refer to [this guide](https://developers.cloudflare.com/dns/zone-setups/full-setup/setup/#update-your-nameservers) on Cloudflare nameservers. However, if you prefer not to use the entire domain, an alternative option is to delegate a subdomain to Cloudflare.

## Input manifest examples

### Load balancing example

!!! warning "Showcase example"
    To make this example functional, you need to specify control plane and node pools. This current showcase will produce an error if used as is.

### Create a secret for Cloudflare and AWS providers
The secret for an Cloudflare provider must include the following mandatory fields: `apitoken`.
```bash
kubectl create secret generic cloudflare-secret-1 --namespace=mynamespace --from-literal=apitoken='kslISA878a6etYAfXYcg5iYyrFGNlCxc'
```

The secret for an AWS provider must include the following mandatory fields: `accesskey` and `secretkey`.
```bash
kubectl create secret generic aws-secret-1 --namespace=mynamespace --from-literal=accesskey='SLDUTKSHFDMSJKDIALASSD' --from-literal=secretkey='iuhbOIJN+oin/olikDSadsnoiSVSDsacoinOUSHD'
```

``` yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: CloudflareExampleManifest
spec:
  providers:
    - name: cloudflare-1
      providerType: cloudflare
      secretRef:
        name: cloudflare-secret-1
        namespace: mynamespace

    - name: aws-1
      providerType: aws
      secretRef:
        name: aws-secret-1
        namespace: mynamespace

  nodePools: 
    dynamic:
      - name: loadbalancer
        providerSpec:
          name: aws-1
          region: eu-central-1
          zone: eu-central-1c
        count: 2
        serverType: t3.medium
        image: ami-0965bd5ba4d59211c

  kubernetes:
    clusters:
      - name: cluster
        version: v1.24.0
        network: 192.168.2.0/24
        pools:
          control: []
          compute: []

  loadBalancers:
    roles:
      - name: apiserver
        protocol: tcp
        port: 6443
        targetPort: 6443
        target: k8sControlPlane

    clusters:
      - name: apiserver-lb-prod
        roles:
          - apiserver
        dns:
          dnsZone: dns-zone
          provider: cloudflare-1
          hostname: my.fancy.url
        targetedK8s: prod-cluster
        pools:
          - loadbalancer-2

```