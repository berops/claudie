# AWS
AWS cloud provider requires you to input the credentials as an `accessKey` and a `secretKey`.

## Compute and DNS example
```yaml
providers:
  aws:
    - name: aws-1
      accessKey: access_key_id
      secretKey: secret_access_key
```

## Create AWS credentials
### Prerequisites

1. Install AWS CLI tools by following [this guide](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html).
2. Setup AWS CLI on your machine by following [this guide](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-quickstart.html).

### Creating AWS credentials for Claudie

1. Create a user using AWS CLI:
    ```bash
    aws iam create-user --user-name claudie
    ```

2. Create a policy document with compute and DNS permissions required by Claudie:
    ```bash
    cat > policy.json <<EOF
    {
       "Version":"2012-10-17",
       "Statement":[
          {
             "Effect":"Allow",
             "Action":[
                "ec2:*"
             ],
             "Resource":"*"
          },
          {
             "Effect":"Allow",
             "Action":[
                "route53:*"
             ],
             "Resource":"*"
          }
       ]
    }
    EOF
    ```

    !!! note "DNS permissions"
        Exclude route53 permissions from the policy document, if you prefer not to use AWS as the DNS provider.

3. Attach the policy to the claudie user:
    ```bash
    aws iam put-user-policy --user-name claudie --policy-name ec2-and-dns-access --policy-document file://policy.json
    ```

4. Create access keys for claudie user:
    ```bash
    aws iam create-access-key --user-name claudie
    ```
    ```json
    {
       "AccessKey":{
          "UserName":"claudie",
          "AccessKeyId":"AKIAIOSFODNN7EXAMPLE",
          "Status":"Active",
          "SecretAccessKey":"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
          "CreateDate":"2018-12-14T17:34:16Z"
       }
    }
    ```

## DNS setup
If you wish to use AWS as your DNS provider where Claudie creates DNS records pointing to Claudie managed clusters, you will need to create a **public hosted zone** by following [this guide](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/CreatingHostedZone.html). 

!!! warning "AWS is not my domain registrar"
    If you haven't acquired a domain via AWS and wish to utilize AWS for hosting your zone, you can refer to [this guide](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/migrate-dns-domain-in-use.html#migrate-dns-change-name-servers-with-provider) on AWS nameservers. However, if you prefer not to use the entire domain, an alternative option is to delegate a subdomain to AWS.

## Input manifest examples
### Single provider, multi region cluster example

```yaml
name: AWSExampleManifest

providers:
  aws:
    - name: aws-1
      # Access key to your AWS account.
      accessKey: SLDUTKSHFDMSJKDIALASSD
      # Secret key to your AWS account.
      secretKey: iuhbOIJN+oin/olikDSadsnoiSVSDsacoinOUSHD

nodePools:
  dynamic:
    - name: control-aws
      providerSpec:
        # Name of the provider instance.
        name: aws-1
        # Region of the nodepool.
        region: eu-central-1
        # Availability zone of the nodepool.
        zone: eu-central-1a
      count: 1
      # Instance type name.
      serverType: t3.medium
      # AMI ID of the image.
      # Make sure to update it according to the region. 
      image: ami-0965bd5ba4d59211c
      
    - name: compute-1-aws
      providerSpec:
        # Name of the provider instance.
        name: aws-1
        # Region of the nodepool.
        region: eu-central-2
        # Availability zone of the nodepool.
        zone: eu-central-2a
      count: 2
      # Instance type name.
      serverType: t3.medium
      # AMI ID of the image.
      # Make sure to update it according to the region. 
      image: ami-0965bd5ba4d59211c
      storageDiskSize: 50

    - name: compute-2-aws
      providerSpec:
        # Name of the provider instance.
        name: aws-1
        # Region of the nodepool.
        region: eu-central-3
        # Availability zone of the nodepool.
        zone: eu-central-3a
      count: 2
      # Instance type name.
      serverType: t3.medium
      # AMI ID of the image.
      # Make sure to update it according to the region. 
      image: ami-0965bd5ba4d59211c
      storageDiskSize: 50

kubernetes:
  clusters:
    - name: aws-cluster
      version: v1.24.0
      network: 192.168.2.0/24
      pools:
        control:
          - control-aws
        compute:
          - compute-1-aws
          - compute-2-aws

```

### Multi provider, multi region clusters example

```yaml
name: AWSExampleManifest

providers:
  aws:
    - name: aws-1
      # Access key to your AWS account.
      accessKey: SLDUTKSHFDMSJKDIALASSD
      # Secret key to your AWS account.
      secretKey: iuhbOIJN+oin/olikDSadsnoiSVSDsacoinOUSHD

    - name: aws-2
      # Access key to your AWS account.
      accessKey: ODURNGUISNFAIPUNUGFINB
      # Secret key to your AWS account.
      secretKey: asduvnva+skd/ounUIBPIUjnpiuBNuNipubnPuip

nodePools:
  dynamic:
    - name: control-aws-1
      providerSpec:
        # Name of the provider instance.
        name: aws-1
        region: eu-central-1
        # Availability zone of the nodepool.
        zone: eu-central-1a
      count: 1
      # Instance type name.
      serverType: t3.medium
      # AMI ID of the image.
      # Make sure to update it according to the region. 
      image: ami-0965bd5ba4d59211c

    - name: control-aws-2
      providerSpec:
        # Name of the provider instance.
        name: aws-2
        # Region of the nodepool.
        region: eu-north-1
        # Availability zone of the nodepool.
        zone: eu-north-1a
      count: 2
      # Instance type name.
      serverType: t3.medium
      # AMI ID of the image.
      # Make sure to update it according to the region. 
      image: ami-03df6dea56f8aa618

    - name: compute-aws-1
      providerSpec:
        # Name of the provider instance.
        name: aws-1
        # Region of the nodepool.
        region: eu-central-2
        # Availability zone of the nodepool.
        zone: eu-central-2a
      count: 2
      # Instance type name.
      serverType: t3.medium
      # AMI ID of the image.
      # Make sure to update it according to the region. 
      image: ami-0965bd5ba4d59211c
      storageDiskSize: 50

    - name: compute-aws-2
      providerSpec:
        # Name of the provider instance.
        name: aws-2
        # Region of the nodepool.
        region: eu-north-3
        # Availability zone of the nodepool.
        zone: eu-north-3a
      count: 2
      # Instance type name.
      serverType: t3.medium
      # AMI ID of the image.
      # Make sure to update it according to the region. 
      image: ami-03df6dea56f8aa618
      storageDiskSize: 50

    - name: loadbalancer-1
      providerSpec:
        # Name of the provider instance.
        name: aws-2
        # Region of the nodepool.
        region: eu-north-3
        # Availability zone of the nodepool.
        zone: eu-north-3a
      count: 2
      # Instance type name.
      serverType: t3.small
      # AMI ID of the image.
      # Make sure to update it according to the region.
      image: ami-03df6dea56f8aa618

kubernetes:
  clusters:
    - name: aws-cluster
      version: v1.24.0
      network: 192.168.2.0/24
      pools:
        control:
          - control-aws-1
          - control-aws-2
        compute:
          - compute-aws-1
          - compute-aws-2
loadBalancers:
  roles:
    - name: apiserver
      protocol: tcp
      port: 6443
      targetPort: 6443
      target: k8sControlPlane

  clusters:
    - name: apiserver-lb-dev
      roles:
        - apiserver
      dns:
        dnsZone: example.com
        provider: aws-2
      targetedK8s: aws-cluster
      pools:
        - loadbalancer-1
```
