# OCI input manifest example

## Single provider, multi region cluster

```yaml
name: OCIExampleManifest

providers:
  oci:
    - name: oci-1
      # private key to the user account
      private_key: |
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
      # fingerprint of the key pair
      key_fingerprint: ab:cd:3f:34:33:22:32:34:54:54:45:76:76:78:98:aa
      # OCID of the tenancy
      tenancy_ocid: ocid2.tenancy.oc2..aaaaaaaayrsfvlvxc34o060kfdygsds21nske76ksjkko21lpsdfsfsgbrtghs
      # OCID of the user
      user_ocid: ocid2.user.oc2..aaaaaaaaaanyrsfvlvxc34o060kfdygsds21nske76ksjkko21lpsdfsf
      # OCID of the compartment
      compartment_ocid: ocid2.compartment.oc2..aaaaaaaaa2rsfvlvxc34o060kfdygsds21nske76ksjkko21lpsdfsf

nodePools:
  dynamic:
    - name: control-oci
      providerSpec:
        # provider instance name
        name: oci-1
        # region of the nodepool
        region: eu-milan-1
        # availability domain of the nodepool
        zone: hsVQ:EU-MILAN-1-AD-1
      count: 1
      # VM shape name
      server_type: VM.Standard2.2
      # OCID of the image
      # make sure to update it according to the region
      image: ocid1.image.oc1.eu-frankfurt-1.aaaaaaaavvsjwcjstxt4sb25na65yx6i34bzdy5oess3pkgwyfa4hxmzpqeq
      disk_size: 50

    - name: compute-1-oci
      providerSpec:
        # provider instance name
        name: oci-1
        # region of the nodepool
        region: eu-frankfurt-1
        # availability domain of the nodepool
        zone: hsVQ:EU-FRANKFURT-1-AD-1
      count: 2
      # VM shape name
      server_type: VM.Standard2.1
      # OCID of the image
      # make sure to update it according to the region
      image: ocid1.image.oc1.eu-frankfurt-1.aaaaaaaavvsjwcjstxt4sb25na65yx6i34bzdy5oess3pkgwyfa4hxmzpqeq
      disk_size: 50

    - name: compute-2-oci
      providerSpec:
        # provider instance name
        name: oci-1
        # region of the nodepool
        region: eu-frankfurt-1
        # availability domain of the nodepool
        zone: hsVQ:EU-FRANKFURT-1-AD-2
      count: 2
      # VM shape name
      server_type: VM.Standard2.1
      # OCID of the image
      # make sure to update it according to the region
      image: ocid1.image.oc1.eu-frankfurt-1.aaaaaaaavvsjwcjstxt4sb25na65yx6i34bzdy5oess3pkgwyfa4hxmzpqeq
      disk_size: 50

kubernetes:
  clusters:
    - name: oci-cluster
      version: v1.23.0
      network: 192.168.2.0/24
      pools:
        control:
          - control-oci
        compute:
          - compute-1-oci
          - compute-2-oci
```

## Multi provider, multi region clusters

```yaml
name: OCIExampleManifest

providers:
  oci:
    - name: oci-1
      # private key to the user account
      private_key: |
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
      # fingerprint of the key pair
      key_fingerprint: ab:cd:3f:34:33:22:32:34:54:54:45:76:76:78:98:aa
      # OCID of the tenancy
      tenancy_ocid: ocid2.tenancy.oc2..aaaaaaaayrsfvlvxc34o060kfdygsds21nske76ksjkko21lpsdfsfsgbrtghs
      # OCID of the user
      user_ocid: ocid2.user.oc2..aaaaaaaaaanyrsfvlvxc34o060kfdygsds21nske76ksjkko21lpsdfsf
      # OCID of the compartment
      compartment_ocid: ocid2.compartment.oc2..aaaaaaaaa2rsfvlvxc34o060kfdygsds21nske76ksjkko21lpsdfsf

    - name: oci-2
      # private key to the user account
      private_key: |
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
      # fingerprint of the key pair
      key_fingerprint: 34:54:54:45:76:76:78:98:aa:ab:cd:3f:34:33:22:32
      # OCID of the tenancy
      tenancy_ocid: ocid2.tenancy.oc2..aaaaaaaayreragzafbdrfedbfdagagrgregagrrgaregfdgvrehdfsfsgbrtghs
      # OCID of the user
      user_ocid: ocid2.user.oc2..aaaaaaaaaanyrsfvlvxc3argaehgaergaregraregaregarsdfsfrgreg2ds
      # OCID of the compartment
      compartment_ocid: ocid2.compartment.oc2..aaaaaaaaa2rsfvlvxc3argaregaregraegzfgragfksjkko21lpsdfsf

nodePools:
  dynamic:
    - name: control-oci-1
      providerSpec:
        # provider instance name
        name: oci-1
        # region of the nodepool
        region: eu-milan-1
        # availability domain of the nodepool
        zone: hsVQ:EU-MILAN-1-AD-1
      count: 1
      # VM shape name
      server_type: VM.Standard2.2
      # OCID of the image
      # make sure to update it according to the region
      image: ocid1.image.oc1.eu-frankfurt-1.aaaaaaaavvsjwcjstxt4sb25na65yx6i34bzdy5oess3pkgwyfa4hxmzpqeq
      disk_size: 50

    - name: control-oci-2
      providerSpec:
        # provider instance name
        name: oci-2
        # region of the nodepool
        region: eu-frankfurt-1
        # availability domain of the nodepool
        zone: hsVQ:EU-FRANKFURT-1-AD-3
      count: 2
      # VM shape name
      server_type: VM.Standard2.1
      # OCID of the image
      # make sure to update it according to the region
      image: ocid1.image.oc1.eu-frankfurt-1.aaaaaaaavvsjwcjstxt4sb25na65yx6i34bzdy5oess3pkgwyfa4hxmzpqeq
      disk_size: 50

    - name: compute-oci-1
      providerSpec:
        # provider instance name
        name: oci-1
        # region of the nodepool
        region: eu-frankfurt-1
        # availability domain of the nodepool
        zone: hsVQ:EU-FRANKFURT-1-AD-1
      count: 2
      # VM shape name
      server_type: VM.Standard2.1
      # OCID of the image
      # make sure to update it according to the region
      image: ocid1.image.oc1.eu-frankfurt-1.aaaaaaaavvsjwcjstxt4sb25na65yx6i34bzdy5oess3pkgwyfa4hxmzpqeq
      disk_size: 50

    - name: compute-oci-2
      providerSpec:
        # provider instance name
        name: oci-2
        # region of the nodepool
        region: eu-milan-1
        # availability domain of the nodepool
        zone: hsVQ:EU-MILAN-1-AD-1
      count: 2
      # VM shape name
      server_type: VM.Standard2.1
      # OCID of the image
      # make sure to update it according to the region
      image: ocid1.image.oc1.eu-frankfurt-1.aaaaaaaavvsjwcjstxt4sb25na65yx6i34bzdy5oess3pkgwyfa4hxmzpqeq
      disk_size: 50

kubernetes:
  clusters:
    - name: oci-cluster
      version: v1.23.0
      network: 192.168.2.0/24
      pools:
        control:
          - control-oci-1
          - control-oci-2
        compute:
          - compute-oci-1
          - compute-oci-2
```
