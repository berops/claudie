# Openstack
Openstack cloud provider requires you to input this authentication details: `authurl`, `domainid`, `projectid`, `applicationcredentialid` and `applicationcredentialsecret`.

## Compute
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: openstack-secret
data:
  authurl: <base64-encoded-auth-url>
  domainid: <base64-encoded-domain-id>
  projectid: <base64-encoded-project-id>
  applicationcredentialid: <base64-encoded-credential-id>
  applicationcredentialsecret: <base64-encoded-credential-secret>
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
| ID           | <your-credential-id>                                                                   |
| Name         | claudie                                                                                |
| Description  | None                                                                                   |
| Project ID   | <your-project-id>                                                                      |
| Roles        | administrator                                                                          |
| Unrestricted | False                                                                                  |
| Access Rules | []                                                                                     |
| Expires At   | None                                                                                   |
| Secret       | <your-credential-secret>                                                               |
+--------------+----------------------------------------------------------------------------------------+
```


## Input manifest examples

### Single provider OVHcloud, multi region cluster example
#### Create a secret for Openstack provider
The secret for an Openstack provider must include the following mandatory fields: `authurl`, `domainid`, `projectid`, `applicationcredentialid` and `applicationcredentialsecret`.

```bash
kubectl create secret generic openstack-secret-1 \
--namespace=<your-namespace> \
--from-literal=authurl='<your-auth-url>' \
--from-literal=domainid='<your-domain-id>' \
--from-literal=projectid='<your-project-id>' \
--from-literal=applicationcredentialid='<your-credential-id>' \
--from-literal=applicationcredentialsecret='<your-credential-secret>'
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
        namespace: <your-namespace>

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
      image: "Ubuntu 24.04"

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
      image: "Ubuntu 24.04"
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
      image: "Ubuntu 24.04"
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
