# AWS
AWS cloud provider requires you to input the credentials as an `accesskey` and a `secretkey`.

## Compute and DNS example
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: aws-secret
data:
  accesskey: U0xEVVRLU0hGRE1TSktESUFMQVNTRA==
  secretkey: aXVoYk9JSk4rb2luL29saWtEU2Fkc25vaVNWU0RzYWNvaW5PVVNIRA==
type: Opaque
```

## Create AWS credentials
### Prerequisites

1. Install AWS CLI tools by following [this guide](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html).
2. Setup AWS CLI on your machine by following [this guide](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-quickstart.html).
3. Ensure that the regions you're planning to use are enabled in your AWS account. You can check the available regions using [this guide](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html#concepts-available-regions), and you can enable them using [this guide](https://docs.aws.amazon.com/accounts/latest/reference/manage-acct-regions.html). Otherwise, you may encounter a misleading error suggesting your STS token is invalid.

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

### Create a secret for AWS provider
The secret for an AWS provider must include the following mandatory fields: `accesskey` and `secretkey`.

```bash
kubectl create secret generic aws-secret-1 --namespace=mynamespace --from-literal=accesskey='SLDUTKSHFDMSJKDIALASSD' --from-literal=secretkey='iuhbOIJN+oin/olikDSadsnoiSVSDsacoinOUSHD'
```

### Single provider, multi region cluster example

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: aws-example-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:

  providers:
    - name: aws-1
      providerType: aws
      secretRef:
        name: aws-secret-1
        namespace: mynamespace

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
        # AMI ID of the image ubuntu 24.04.
        # Make sure to update it according to the region.
        image: ami-07eef52105e8a2059

      - name: compute-1-aws
        providerSpec:
          # Name of the provider instance.
          name: aws-1
          # Region of the nodepool.
          region: eu-west-2
          # Availability zone of the nodepool.
          zone: eu-west-2a
        count: 2
        # Instance type name.
        serverType: t3.medium
        # AMI ID of the image ubuntu 24.04.
        # Make sure to update it according to the region.
        image: ami-091f18e98bc129c4e
        storageDiskSize: 50

      - name: compute-2-aws
        providerSpec:
          # Name of the provider instance.
          name: aws-1
          # Region of the nodepool.
          region: eu-west-2
          # Availability zone of the nodepool.
          zone: eu-west-2a
        count: 2
        # Instance type name.
        serverType: t3.medium
        # AMI ID of the image ubuntu 24.04.
        # Make sure to update it according to the region.
        image: ami-091f18e98bc129c4e
        storageDiskSize: 50

  kubernetes:
    clusters:
      - name: aws-cluster
        version: v1.31.0
        network: 192.168.2.0/24
        pools:
          control:
            - control-aws
          compute:
            - compute-1-aws
            - compute-2-aws

```

### Multi provider, multi region clusters example

```bash
kubectl create secret generic aws-secret-1 --namespace=mynamespace --from-literal=accesskey='SLDUTKSHFDMSJKDIALASSD' --from-literal=secretkey='iuhbOIJN+oin/olikDSadsnoiSVSDsacoinOUSHD'
kubectl create secret generic aws-secret-2 --namespace=mynamespace --from-literal=accesskey='ODURNGUISNFAIPUNUGFINB' --from-literal=secretkey='asduvnva+skd/ounUIBPIUjnpiuBNuNipubnPuip'
```

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: aws-example-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:

  providers:
    - name: aws-1
      providerType: aws
      secretRef:
        name: aws-secret-1
        namespace: mynamespace
    - name: aws-2
      providerType: aws
      secretRef:
        name: aws-secret-2
        namespace: mynamespace

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
        # AMI ID of the image ubuntu 24.04.
        # Make sure to update it according to the region.
        image: ami-07eef52105e8a2059

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
        # AMI ID of the image ubuntu 24.04.
        # Make sure to update it according to the region.
        image: ami-09a9858973b288bdd

      - name: compute-aws-1
        providerSpec:
          # Name of the provider instance.
          name: aws-1
          # Region of the nodepool.
          region: eu-central-1
          # Availability zone of the nodepool.
          zone: eu-central-1a
        count: 2
        # Instance type name.
        serverType: t3.medium
        # AMI ID of the image ubuntu 24.04.
        # Make sure to update it according to the region.
        image: ami-07eef52105e8a2059
        storageDiskSize: 50

      - name: compute-aws-2
        providerSpec:
          # Name of the provider instance.
          name: aws-2
          # Region of the nodepool.
          region: eu-west-3
          # Availability zone of the nodepool.
          zone: eu-west-3a
        count: 2
        # Instance type name.
        serverType: t3.medium
        # AMI ID of the image ubuntu 24.04.
        # Make sure to update it according to the region.
        image: ami-06e02ae7bdac6b938
        storageDiskSize: 50

  kubernetes:
    clusters:
      - name: aws-cluster
        version: v1.31.0
        network: 192.168.2.0/24
        pools:
          control:
            - control-aws-1
            - control-aws-2
          compute:
            - compute-aws-1
            - compute-aws-2
```
