# OCI
OCI provider requires you to input `privateKey`, `keyFingerprint`, `tenancyOcid`, `userOcid`, and `compartmentOcid`.

## Compute and DNS example

```yaml
providers:
  oci:
    - name: oci-1
      privateKey: private_key
      keyFingerprint: fingerprint_placeholder
      tenancyOcid: tenancy_ocid
      userOcid: user_ocid
      compartmentOcid: compartment_ocid
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

12. You can either manually perform this step or use the following script with the [provided template](#compute-and-dns-example) to safely replace the credentials field with your generated credentials:
```bash
{
  compartment_ocid=$(oci iam compartment list | jq -r '.data[] | select(.name == "claudie-compartment") | .id')
  fingerprint=$(oci iam user api-key list --user-id $user_ocid | jq -r '.data[0].fingerprint')
  yq '.providers.oci[0].privateKey = load_str("claudie-user.pem")' template.yaml > oci.yaml
  sed -i "s/fingerprint_placeholder/$fingerprint/g" oci.yaml
  sed -i "s/tenancy_ocid/$tenancy_ocid/g" oci.yaml
  sed -i "s/user_ocid/$user_ocid/g" oci.yaml
  sed -i "s/compartment_ocid/$compartment_ocid/g" oci.yaml
}
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

```yaml
name: OCIExampleManifest

providers:
  oci:
    - name: oci-1
      # Private key to the user account.
      privateKey: |
        -----BEGIN RSA PRIVATE KEY-----
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/==
        -----END RSA PRIVATE KEY-----
      # Fingerprint of the key pair.
      keyFingerprint: ab:cd:3f:34:33:22:32:34:54:54:45:76:76:78:98:aa
      # OCID of the tenancy.
      tenancyOcid: ocid2.tenancy.oc2..aaaaaaaayrsfvlvxc34o060kfdygsds21nske76ksjkko21lpsdfsfsgbrtghs
      # OCID of the user.
      userOcid: ocid2.user.oc2..aaaaaaaaaanyrsfvlvxc34o060kfdygsds21nske76ksjkko21lpsdfsf
      # OCID of the compartment.
      compartmentOcid: ocid2.compartment.oc2..aaaaaaaaa2rsfvlvxc34o060kfdygsds21nske76ksjkko21lpsdfsf

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

```yaml
name: OCIExampleManifest

providers:
  oci:
    - name: oci-1
      # Private key to the user account.
      privateKey: |
        -----BEGIN RSA PRIVATE KEY-----
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/==
        -----END RSA PRIVATE KEY-----
      # Fingerprint of the key pair.
      keyFingerprint: ab:cd:3f:34:33:22:32:34:54:54:45:76:76:78:98:aa
      # OCID of the tenancy.
      tenancyOcid: ocid2.tenancy.oc2..aaaaaaaayrsfvlvxc34o060kfdygsds21nske76ksjkko21lpsdfsfsgbrtghs
      # OCID of the user.
      userOcid: ocid2.user.oc2..aaaaaaaaaanyrsfvlvxc34o060kfdygsds21nske76ksjkko21lpsdfsf
      # OCID of the compartment.
      compartmentOcid: ocid2.compartment.oc2..aaaaaaaaa2rsfvlvxc34o060kfdygsds21nske76ksjkko21lpsdfsf

    - name: oci-2
      # Private key to the user account.
      privateKey: |
        -----BEGIN RSA PRIVATE KEY-----
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        IUBJNINoisdncNIUBNNpniuniupNPIUNuipbnPIUNPIUBSNUPIbnui/OUINNPOIn
        MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/==
        -----END RSA PRIVATE KEY-----
      # Fingerprint of the key pair.
      keyFingerprint: 34:54:54:45:76:76:78:98:aa:ab:cd:3f:34:33:22:32
      # OCID of the tenancy.
      tenancyOcid: ocid2.tenancy.oc2..aaaaaaaayreragzafbdrfedbfdagagrgregagrrgaregfdgvrehdfsfsgbrtghs
      # OCID of the user.
      userOcid: ocid2.user.oc2..aaaaaaaaaanyrsfvlvxc3argaehgaergaregraregaregarsdfsfrgreg2ds
      # OCID of the compartment.
      compartmentOcid: ocid2.compartment.oc2..aaaaaaaaa2rsfvlvxc3argaregaregraegzfgragfksjkko21lpsdfsf

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

    - name: loadbalancer-1
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
        provider: oci-2
      targetedK8s: oci-cluster
      pools:
        - loadbalancer-1
```
