# OCI
OCI provider requires you to input `privatekey`, `keyfingerprint`, `tenancyocid`, `userocid`, and `compartmentocid`.

## Compute and DNS example

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: oci-secret
data:
  compartmentocid: b2NpZDIuY29tcGFydG1lbnQub2MyLi5hYWFhYWFhYWEycnNmdmx2eGMzNG8wNjBrZmR5Z3NkczIxbnNrZTc2a3Nqa2tvMjFscHNkZnNm    
  keyfingerprint: YWI6Y2Q6M2Y6MzQ6MzM6MjI6MzI6MzQ6NTQ6NTQ6NDU6NzY6NzY6Nzg6OTg6YWE=
  privatekey: >-
    LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQogICAgICAgIE1JSUV2UUlCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktjd2dnU2pBZ0VBQW9JQkFRQ2oyL2Fza0pTTG9zYWQKICAgICAgICBNSUlFdlFJQkFEQU5CZ2txaGtpRzl3MEJBUUVGQUFTQ0JLY3dnZ1NqQWdFQUFvSUJBUUNqMi9hc2tKU0xvc2FkCiAgICAgICAgTUlJRXZRSUJBREFOQmdrcWhraUc5dzBCQVFFRkFBU0NCS2N3Z2dTakFnRUFBb0lCQVFDajIvYXNrSlNMb3NhZAogICAgICAgIE1JSUV2UUlCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktjd2dnU2pBZ0VBQW9JQkFRQ2oyL2Fza0pTTG9zYWQKICAgICAgICBNSUlFdlFJQkFEQU5CZ2txaGtpRzl3MEJBUUVGQUFTQ0JLY3dnZ1NqQWdFQUFvSUJBUUNqMi9hc2tKU0xvc2FkCiAgICAgICAgTUlJRXZRSUJBREFOQmdrcWhraUc5dzBCQVFFRkFBU0NCS2N3Z2dTakFnRUFBb0lCQVFDajIvYXNrSlNMb3NhZAogICAgICAgIE1JSUV2UUlCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktjd2dnU2pBZ0VBQW9JQkFRQ2oyL2Fza0pTTG9zYWQKICAgICAgICBNSUlFdlFJQkFEQU5CZ2txaGtpRzl3MEJBUUVGQUFTQ0JLY3dnZ1NqQWdFQUFvSUJBUUNqMi9hc2tKU0xvc2FkCiAgICAgICAgTUlJRXZRSUJBREFOQmdrcWhraUc5dzBCQVFFRkFBU0NCS2N3Z2dTakFnRUFBb0lCQVFDajIvYXNrSlNMb3NhZAogICAgICAgIE1JSUV2UUlCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktjd2dnU2pBZ0VBQW9JQkFRQ2oyL2Fza0pTTG9zYWQKICAgICAgICBNSUlFdlFJQkFEQU5CZ2txaGtpRzl3MEJBUUVGQUFTQ0JLY3dnZ1NqQWdFQUFvSUJBUUNqMi9hc2tKU0xvc2FkCiAgICAgICAgTUlJRXZRSUJBREFOQmdrcWhraUc5dzBCQVFFRkFBU0NCS2N3Z2dTakFnRUFBb0lCQVFDajIvYXNrSlNMb3NhZAogICAgICAgIE1JSUV2UUlCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktjd2dnU2pBZ0VBQW9JQkFRQ2oyL2Fza0pTTG9zYWQKICAgICAgICBNSUlFdlFJQkFEQU5CZ2txaGtpRzl3MEJBUUVGQUFTQ0JLY3dnZ1NqQWdFQUFvSUJBUUNqMi9hc2tKU0xvc2FkCiAgICAgICAgTUlJRXZRSUJBREFOQmdrcWhraUc5dzBCQVFFRkFBU0NCS2N3Z2dTakFnRUFBb0lCQVFDajIvYXNrSlNMb3NhZAogICAgICAgIE1JSUV2UUlCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktjd2dnU2pBZ0VBQW9JQkFRQ2oyL2Fza0pTTG9zYWQKICAgICAgICBNSUlFdlFJQkFEQU5CZ2txaGtpRzl3MEJBUUVGQUFTQ0JLY3dnZ1NqQWdFQUFvSUJBUUNqMi9hc2tKU0xvc2FkCiAgICAgICAgTUlJRXZRSUJBREFOQmdrcWhraUc5dzBCQVFFRkFBU0NCS2N3Z2dTakFnRUFBb0lCQVFDajIvYXNrSlNMb3NhZAogICAgICAgIE1JSUV2UUlCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktjd2dnU2pBZ0VBQW9JQkFRQ2oyL2Fza0pTTG9zYWQKICAgICAgICBNSUlFdlFJQkFEQU5CZ2txaGtpRzl3MEJBUUVGQUFTQ0JLY3dnZ1NqQWdFQUFvSUJBUUNqMi9hc2tKU0xvc2FkCiAgICAgICAgTUlJRXZRSUJBREFOQmdrcWhraUc5dzBCQVFFRkFBU0NCS2N3Z2dTakFnRUFBb0lCQVFDajIvYXNrSlNMb3NhZAogICAgICAgIE1JSUV2UUlCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktjd2dnU2pBZ0VBQW9JQkFRQ2oyL2Fza0pTTG9zYWQKICAgICAgICBNSUlFdlFJQkFEQU5CZ2txaGtpRzl3MEJBUUVGQUFTQ0JLY3dnZ1NqQWdFQUFvSUJBUUNqMi9hc2tKU0xvc2FkCiAgICAgICAgTUlJRXZRSUJBREFOQmdrcWhraUc5dzBCQVFFRkFBU0NCS2N3Z2dTakFnRUFBb0lCQVFDajIvYXNrSlNMb3NhZAogICAgICAgIE1JSUV2UUlCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktjd2dnU2pBZ0VBQW9JQkFRQ2oyLz09CiAgICAgICAgLS0tLS1FTkQgUlNBIFBSSVZBVEUgS0VZLS0tLS0=
  tenancyocid: b2NpZDIudGVuYW5jeS5vYzIuLmFhYWFhYWFheXJzZnZsdnhjMzRvMDYwa2ZkeWdzZHMyMW5za2U3NmtzamtrbzIxbHBzZGZzZnNnYnJ0Z2hz
  userocid: b2NpZDIudXNlci5vYzIuLmFhYWFhYWFhYWFueXJzZnZsdnhjMzRvMDYwa2ZkeWdzZHMyMW5za2U3NmtzamtrbzIxbHBzZGZzZg==
type: Opaque
```

## Create OCI credentials
### Prerequisites
1. Install OCI CLI by following [this guide](https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/cliinstall.htm).
2. Configure OCI CLI by following [this guide](https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/cliconfigure.htm).

### Creating OCI credentials for Claudie

1. Export your tenant id:
```bash
export tenancy_ocid="ocid"
```

    !!! note "Find your tenant id"
        You can find it under `Identity & Security` tab and `Compartments` option.

2. Create OCI compartment where Claudie deploys its resources:
```bash
{
  oci iam compartment create --name claudie-compartment --description claudie-compartment --compartment-id $tenancy_ocid
}
```

4. Create the claudie user:
```bash
oci iam user create --name claudie-user --compartment-id $tenancy_ocid --description claudie-user --email <email address>
```

5. Create a group that will hold permissions for the user:
```bash
oci iam group create --name claudie-group --compartment-id $tenancy_ocid --description claudie-group
```

6. Generate policy file with necessary permissions:
```bash
{
cat > policy.txt <<EOF
[
  "Allow group claudie-group to manage instance-family in compartment claudie-compartment",
  "Allow group claudie-group to manage volume-family in compartment claudie-compartment",
  "Allow group claudie-group to manage virtual-network-family in tenancy",
  "Allow group claudie-group to manage dns-zones in compartment claudie-compartment",
  "Allow group claudie-group to manage dns-records in compartment claudie-compartment"
]
EOF
}
```

7. Create a policy with required permissions:
```bash
oci iam policy create --name claudie-policy --statements file://policy.txt --compartment-id $tenancy_ocid --description claudie-policy
```

8. Declare `user_ocid` and `group_ocid`:
```bash
{
  group_ocid=$(oci iam group list | jq -r '.data[] | select(.name == "claudie-group") | .id')
  user_ocid=$(oci iam user list | jq -r '.data[] | select(.name == "claudie-user") | .id')
}
```

9. Attach claudie-user to claudie-group:
```bash
oci iam group add-user --group-id $group_ocid --user-id $user_ocid
```

10. Generate key pair for claudie-user and enter `N/A` for no passphrase:
```bash
oci setup keys --key-name claudie-user --output-dir .
```

11. Upload the public key to use for the claudie-user:
```bash
oci iam user api-key upload --user-id $user_ocid --key-file claudie-user_public.pem
```

12. Export `compartment_ocid` and `fingerprint`, to use them when creating provider secret.
```bash
  compartment_ocid=$(oci iam compartment list | jq -r '.data[] | select(.name == "claudie-compartment") | .id')
  fingerprint=$(oci iam user api-key list --user-id $user_ocid | jq -r '.data[0].fingerprint')
```

## DNS setup
If you wish to use OCI as your DNS provider where Claudie creates DNS records pointing to Claudie managed clusters, you will need to create a **public DNS zone** by following [this guide](https://docs.oracle.com/en-us/iaas/Content/DNS/Concepts/gettingstarted_topic-Creating_a_Zone.htm#top).

!!! note "OCI is not my domain registrar"
    You cannot buy a domain from Oracle at this time so you can update nameservers of your OCI hosted zone by following [this guide](https://blogs.oracle.com/cloud-infrastructure/post/bring-your-domain-name-to-oracle-cloud-infrastructures-edge-services) on changing nameservers. However, if you prefer not to use the entire domain, an alternative option is to delegate a subdomain to OCI.

## IAM policies required by Claudie

```tf
"Allow group <GROUP_NAME> to manage instance-family in compartment <COMPARTMENT_NAME>"
"Allow group <GROUP_NAME> to manage volume-family in compartment <COMPARTMENT_NAME>"
"Allow group <GROUP_NAME> to manage virtual-network-family in tenancy"
"Allow group <GROUP_NAME> to manage dns-zones in compartment <COMPARTMENT_NAME>",
"Allow group <GROUP_NAME> to manage dns-records in compartment <COMPARTMENT_NAME>",
```

## Input manifest examples
### Single provider, multi region cluster example

#### Create a secret for OCI provider
The secret for an OCI provider must include the following mandatory fields: `compartmentocid`, `userocid`, `tenancyocid`, `keyfingerprint` and `privatekey`.

```bash
# Refer to values exported in "Creating OCI credentials for Claudie" section
kubectl create secret generic oci-secret-1 --namespace=mynamespace --from-literal=compartmentocid=$compartment_ocid --from-literal=userocid=$user_ocid --from-literal=tenancyocid=$tenancy_ocid --from-literal=keyfingerprint=$fingerprint --from-file=privatekey=./claudie-user_public.pem
```

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: OCIExampleManifest
spec:
  providers:
    - name: oci-1
      providerType: oci
      secretRef:
        name: oci-secret-1
        namespace: mynamespace

  nodePools:
    dynamic:
      - name: control-oci
        providerSpec:
          # Name of the provider instance.
          name: oci-1
          # Region of the nodepool.
          region: eu-milan-1
          # Availability domain of the nodepool.
          zone: hsVQ:EU-MILAN-1-AD-1
        count: 1
        # VM shape name.
        serverType: VM.Standard2.2
        # OCID of the image.
        # Make sure to update it according to the region.
        image: ocid1.image.oc1.eu-frankfurt-1.aaaaaaaavvsjwcjstxt4sb25na65yx6i34bzdy5oess3pkgwyfa4hxmzpqeq

      - name: compute-1-oci
        providerSpec:
          # Name of the provider instance.
          name: oci-1
          # Region of the nodepool.
          region: eu-frankfurt-1
          # Availability domain of the nodepool.
          zone: hsVQ:EU-FRANKFURT-1-AD-1
        count: 2
        # VM shape name.
        serverType: VM.Standard2.1
        # OCID of the image.
        # Make sure to update it according to the region.
        image: ocid1.image.oc1.eu-frankfurt-1.aaaaaaaavvsjwcjstxt4sb25na65yx6i34bzdy5oess3pkgwyfa4hxmzpqeq
        storageDiskSize: 50

      - name: compute-2-oci
        providerSpec:
          # Name of the provider instance.
          name: oci-1
          # Region of the nodepool.
          region: eu-frankfurt-1
          # Availability domain of the nodepool.
          zone: hsVQ:EU-FRANKFURT-1-AD-2
        count: 2
        # VM shape name.
        serverType: VM.Standard2.1
        # OCID of the image.
        # Make sure to update it according to the region.
        image: ocid1.image.oc1.eu-frankfurt-1.aaaaaaaavvsjwcjstxt4sb25na65yx6i34bzdy5oess3pkgwyfa4hxmzpqeq
        storageDiskSize: 50

  kubernetes:
    clusters:
      - name: oci-cluster
        version: v1.24.0
        network: 192.168.2.0/24
        pools:
          control:
            - control-oci
          compute:
            - compute-1-oci
            - compute-2-oci
```

### Multi provider, multi region clusters example
#### Create a secret for OCI provider
The secret for an OCI provider must include the following mandatory fields: `compartmentocid`, `userocid`, `tenancyocid`, `keyfingerprint` and `privatekey`.

```bash
# Refer to values exported in "Creating OCI credentials for Claudie" section
kubectl create secret generic oci-secret-1 --namespace=mynamespace --from-literal=compartmentocid=$compartment_ocid --from-literal=userocid=$user_ocid --from-literal=tenancyocid=$tenancy_ocid --from-literal=keyfingerprint=$fingerprint --from-file=privatekey=./claudie-user_public.pem

kubectl create secret generic oci-secret-1 --namespace=mynamespace --from-literal=compartmentocid=$compartment_ocid2 --from-literal=userocid=$user_ocid2 --from-literal=tenancyocid=$tenancy_ocid2 --from-literal=keyfingerprint=$fingerprint2 --from-file=privatekey=./claudie-user_public2.pem
```

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: OCIExampleManifest
spec:
  providers:
    - name: oci-1
      providerType: oci
      secretRef:
        name: oci-secret-1
        namespace: mynamespace
    - name: oci-2
      providerType: oci
      secretRef:
        name: oci-secret-2
        namespace: mynamespace

  nodePools:
    dynamic:
      - name: control-oci-1
        providerSpec:
          # Name of the provider instance.
          name: oci-1
          # Region of the nodepool.
          region: eu-milan-1
          # Availability domain of the nodepool.
          zone: hsVQ:EU-MILAN-1-AD-1
        count: 1
        # VM shape name.
        serverType: VM.Standard2.2
        # OCID of the image.
        # Make sure to update it according to the region.
        image: ocid1.image.oc1.eu-frankfurt-1.aaaaaaaavvsjwcjstxt4sb25na65yx6i34bzdy5oess3pkgwyfa4hxmzpqeq

      - name: control-oci-2
        providerSpec:
          # Name of the provider instance.
          name: oci-2
          # Region of the nodepool.
          region: eu-frankfurt-1
          # Availability domain of the nodepool.
          zone: hsVQ:EU-FRANKFURT-1-AD-3
        count: 2
        # VM shape name.
        serverType: VM.Standard2.1
        # OCID of the image.
        # Make sure to update it according to the region.
        image: ocid1.image.oc1.eu-frankfurt-1.aaaaaaaavvsjwcjstxt4sb25na65yx6i34bzdy5oess3pkgwyfa4hxmzpqeq

      - name: compute-oci-1
        providerSpec:
          # Name of the provider instance.
          name: oci-1
          # Region of the nodepool.
          region: eu-frankfurt-1
          # Availability domain of the nodepool.
          zone: hsVQ:EU-FRANKFURT-1-AD-1
        count: 2
        # VM shape name.
        serverType: VM.Standard2.1
        # OCID of the image.
        # Make sure to update it according to the region.
        image: ocid1.image.oc1.eu-frankfurt-1.aaaaaaaavvsjwcjstxt4sb25na65yx6i34bzdy5oess3pkgwyfa4hxmzpqeq
        storageDiskSize: 50

      - name: compute-oci-2
        providerSpec:
          # Name of the provider instance.
          name: oci-2
          # Region of the nodepool.
          region: eu-milan-1
          # Availability domain of the nodepool.
          zone: hsVQ:EU-MILAN-1-AD-1
        count: 2
        # VM shape name.
        serverType: VM.Standard2.1
        # OCID of the image.
        # Make sure to update it according to the region..
        image: ocid1.image.oc1.eu-frankfurt-1.aaaaaaaavvsjwcjstxt4sb25na65yx6i34bzdy5oess3pkgwyfa4hxmzpqeq
        storageDiskSize: 50

  kubernetes:
    clusters:
      - name: oci-cluster
        version: v1.24.0
        network: 192.168.2.0/24
        pools:
          control:
            - control-oci-1
            - control-oci-2
          compute:
            - compute-oci-1
            - compute-oci-2
```
