# AWS input manifest example

## Single provider, multi region cluster

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
      diskSize: 50

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
      diskSize: 50

kubernetes:
  clusters:
    - name: aws-cluster
      version: v1.23.0
      network: 192.168.2.0/24
      pools:
        control:
          - control-aws
        compute:
          - compute-1-aws
          - compute-2-aws

```

## Multi provider, multi region clusters

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
      diskSize: 50

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
      diskSize: 50

kubernetes:
  clusters:
    - name: aws-cluster
      version: v1.23.0
      network: 192.168.2.0/24
      pools:
        control:
          - control-aws-1
          - control-aws-2
        compute:
          - compute-aws-1
          - compute-aws-2
```
