# Openstack
Openstack cloud provider requires you to input this authentication details: `authurl`, `domainid`, `projectid`, `applicationcredentialid` and `applicationcredentialsecret`.

## Compute
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: openstack-secret
data:
  authurl: U0xEVVRLU0hGRE1TSktESUFMQVNTRA==
  domainid: ZGVmYXVsdAo=
  projectid: OGM1MDZmZjBhNmQzNGVkNzkyNTBkZWQ4OGRhNzBmNmEK
  applicationcredentialid: YmFhMDkxYTYyNWJkNGKyNjlmNzA5Mzc5ODg4YTQ5YzMQ
  applicationcredentialsecret: YndNRUVLMmNPdE5oSDlJbXIzRmRlVEVPTG9odU1HcUQzVUxSTzgzWjZaTXh0U3hSSXNVLWNkTHlN==
type: Opaque
```

## Create Openstack API credentials
You can create Openstack API credentials by following [this guide](https://docs.openstack.org/python-openstackclient/latest/cli/command-objects/application-credentials.html). 


The application credentials must have permissions to create, modify, and delete the following resources:
```text
Instances (servers)
Volumes
Networks and subnets
Routers and floating IPs
Security groups and security group rules
Additionally, the credentials should be able to attach floating IPs, volumes, and networks to instances, as well as add tags to resources.
```

!!! note "Permissions required may vary between OpenStack providers."
    In most cases, the member or creator role is sufficient. However, some providers may require assigning a higher-privileged role to the application credential for full access.
    For specific permission requirements, please refer to your OpenStack provider's documentation.

```bash
openstack application credential create --role administrator claudie 
+--------------+----------------------------------------------------------------------------------------+
| Field        | Value                                                                                  |
+--------------+----------------------------------------------------------------------------------------+
| ID           | 296f552c62f2443985b57b0280a5ca74                                                       |
| Name         | claudie                                                                                |
| Description  | None                                                                                   |
| Project ID   | 8c506ff0a6d34ed79250ded88da70f6a                                                       |
| Roles        | administrator                                                                          |
| Unrestricted | False                                                                                  |
| Access Rules | []                                                                                     |
| Expires At   | None                                                                                   |
| Secret       | _0ZTJxyQFEOg9_sAMmeEvxDAEkI_vxoF1VYu-wGiXRCE_XgxIXxE9XxYfDtTNTqh4TXCfsP5qANljTfBZ0bsHQ |
+--------------+----------------------------------------------------------------------------------------+
```


## Input manifest examples

### Single provider OVHcloud, multi region cluster example
#### Create a secret for Openstack provider
The secret for an Openstack provider must include the following mandatory fields: `authurl`, `domainid`, `projectid`, `applicationcredentialid` and `applicationcredentialsecret`.

```bash
kubectl create secret generic openstack-secret-1 \
--namespace=mynamespace \
--from-literal=authurl='https://auth.cloud.ovh.net' \
--from-literal=domainid='default' \
--from-literal=projectid='8c506ff0a6d34ed79250ded88da70f6a' \
--from-literal=applicationcredentialid='5533f69597734911921a7ee3f30c6464' \
--from-literal=applicationcredentialsecret='IdtoVmeRC_O-SClReHX9mzo4PRYvyVwQqWNBmWg2XIDGEA_CvhlVaObMEo2-BoH7GgpZZGhY_aqFgHh63NrMKw'
```

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: openstack-example-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: openstack-1
      providerType: openstack
      secretRef:
        name: openstack-secret-1
        namespace: mynamespace

  nodePools:
    dynamic:
    - name: control-os
      providerSpec:
        # Name of the provider instance.
        name: openstack-1
        # Region of the nodepool.
        region: WAW1
        # Zone of the region.
        zone: nova
        # External network name within zone.
        externalNetworkName: Ext-Net
      count: 1
      # Machine type name.
      serverType: c3-4-flex
      # OS image name.
      image: fa62ad8e-5df1-4f35-8329-0ba96a5426ed

    - name: compute-1-os
      providerSpec:
        # Name of the provider instance.
        name: openstack-1
        # Region of the nodepool.
        region: GRA9
        # Zone of the region.
        zone: nova
        # External network name within zone.
        externalNetworkName: Ext-Net
      count: 1
      # Machine type name.
      serverType: c3-4-flex
      # OS image name.
      image: 2053eb45-d392-460a-bcb1-abc9af4f3924
      storageDiskSize: 50

    - name: compute-2-os
      providerSpec:
        # Name of the provider instance
        name: openstack-1
        # Region of the nodepool.
        region: RBX-A
        # Zone of the region.
        zone: nova
        # External network name within zone.
        externalNetworkName: Ext-Net
      count: 1
      # Machine type name.
      serverType: c3-4-flex
      # OS image name.
      image: fde01fb6-68e9-4b81-8299-c2caad6ff915
      storageDiskSize: 50

  kubernetes:
    clusters:
      - name: brando-test
        version: "v1.31.0"
        network: 192.168.2.0/24
        pools:
          control:
            - control-os
          compute:
            - compute-1-os
            - compute-2-os
```