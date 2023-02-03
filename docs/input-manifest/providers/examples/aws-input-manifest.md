# AWS input manifest example

## Single provider, multi region cluster

```yaml
name: AWSExampleManifest

providers:
  aws:
    - name: aws-1
      # access key to your AWS account
      access_key: SLDUTKSHFDMSJKDIALASSD
      # secret key to your AWS account
      secret_key: iuhbOIJN+oin/olikDSadsnoiSVSDsacoinOUSHD

nodePools:
  dynamic:
    - name: control-aws
      providerSpec:
        # name of the provider instance
        name: aws-1
        # region of the nodepool
        region: eu-central-1
        # availability zone of the nodepool
        zone: eu-central-1a
      count: 1
      # instance type name
      server_type: t3.medium
      # ami ID of the image
      # make sure to update it according to the region 
      image: ami-0965bd5ba4d59211c
      disk_size: 50
      
    - name: compute-1-aws
      providerSpec:
        # name of the provider instance
        name: aws-1
        # region of the nodepool
        region: eu-central-2
        # availability zone of the nodepool
        zone: eu-central-2a
      count: 2
      # instance type name
      server_type: t3.medium
      # ami ID of the image
      # make sure to update it according to the region 
      image: ami-0965bd5ba4d59211c

    - name: compute-2-aws
      providerSpec:
        # name of the provider instance
        name: aws-1
        # region of the nodepool
        region: eu-central-3
        # availability zone of the nodepool
        zone: eu-central-3a
      count: 2
      # instance type name
      server_type: t3.medium
      # ami ID of the image
      # make sure to update it according to the region 
      image: ami-0965bd5ba4d59211c

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
      # access key to your AWS account
      access_key: SLDUTKSHFDMSJKDIALASSD
      # secret key to your AWS account
      secret_key: iuhbOIJN+oin/olikDSadsnoiSVSDsacoinOUSHD

    - name: aws-2
      # access key to your AWS account
      access_key: ODURNGUISNFAIPUNUGFINB
      # secret key to your AWS account
      secret_key: asduvnva+skd/ounUIBPIUjnpiuBNuNipubnPuip

nodePools:
  dynamic:
    - name: control-aws-1
      providerSpec:
        # name of the provider instance
        name: aws-1
        region: eu-central-1
        # availability zone of the nodepool
        zone: eu-central-1a
      count: 1
      # instance type name
      server_type: t3.medium
      # ami ID of the image
      # make sure to update it according to the region 
      image: ami-0965bd5ba4d59211c
      disk_size: 50

    - name: control-aws-2
      providerSpec:
        # name of the provider instance
        name: aws-2
        # region of the nodepool
        region: eu-north-1
        # availability zone of the nodepool
        zone: eu-north-1a
      count: 2
      # instance type name
      server_type: t3.medium
      # ami ID of the image
      # make sure to update it according to the region 
      image: ami-03df6dea56f8aa618
      disk_size: 50

    - name: compute-aws-1
      providerSpec:
        # name of the provider instance
        name: aws-1
        # region of the nodepool
        region: eu-central-2
        # availability zone of the nodepool
        zone: eu-central-2a
      count: 2
      # instance type name
      server_type: t3.medium
      # ami ID of the image
      # make sure to update it according to the region 
      image: ami-0965bd5ba4d59211c

    - name: compute-aws-2
      providerSpec:
        # name of the provider instance
        name: aws-2
        # region of the nodepool
        region: eu-north-3
        # availability zone of the nodepool
        zone: eu-north-3a
      count: 2
      # instance type name
      server_type: t3.medium
      # ami ID of the image
      # make sure to update it according to the region 
      image: ami-03df6dea56f8aa618

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
