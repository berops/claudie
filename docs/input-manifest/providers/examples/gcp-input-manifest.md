# GCP input manifest example

## Single provider, multi region cluster

```yaml
name: GCPExampleManifest

providers:
  gcp:
    - name: gcp-1
      # GCP project for the service account.
      gcpProject: project-claudie
      # Service account key.
      credentials: |
        {
        "type": "service_account",
        "project_id": "project-claudie",
        "private_key_id": "bskdlo875s9087394763eb84e407903lskdimp439",
        "private_key": "-----BEGIN PRIVATE KEY-----\nSKLOosKJUSDANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhki\n-----END PRIVATE KEY-----\n",
        "client_email": "claudie@project-claudie-123456.iam.gserviceaccount.com",
        "client_id": "109876543211234567890","auth_uri": "https://accounts.google.com/o/oauth2/auth",
        "token_uri": "https://oauth2.googleapis.com/token",
        "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
        "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/claudie%40claudie-project-123456.iam.gserviceaccount.com"
        }

nodePools:
  dynamic:
    - name: control-gcp
      providerSpec:
        # Name of the provider instance.
        name: gcp-1
        # Region of the nodepool.
        region: europe-west1
        # Zone of the nodepool.
        zone: europe-west1-c
      count: 1
      # Machine type name.
      serverType: e2-medium
      # OS image name.
      image: ubuntu-os-cloud/ubuntu-2204-jammy-v20221206

    - name: compute-1-gcp
      providerSpec:
        # Name of the provider instance.
        name: gcp-1
        # Region of the nodepool.
        region: europe-west3
        # Zone of the nodepool.
        zone: europe-west3-a
      count: 2
      # Machine type name.
      serverType: e2-medium
      # OS image name.
      image: ubuntu-os-cloud/ubuntu-2204-jammy-v20221206
      storageDiskSize: 50

    - name: compute-2-gcp
      providerSpec:
        # Name of the provider instance.
        name: gcp-1
        # Region of the nodepool.
        region: europe-west2
        # Zone of the nodepool.
        zone: europe-west2-a
      count: 2
      # Machine type name.
      serverType: e2-medium
      # OS image name.
      image: ubuntu-os-cloud/ubuntu-2204-jammy-v20221206
      storageDiskSize: 50

kubernetes:
  clusters:
    - name: gcp-cluster
      version: v1.23.0
      network: 192.168.2.0/24
      pools:
        control:
          - control-gcp
        compute:
          - compute-1-gcp
          - compute-2-gcp
```

## Multi provider, multi region clusters

```yaml
name: GCPExampleManifest

providers:
  gcp:
    - name: gcp-1
      # GCP project for the service account.
      gcpProject: project-claudie-1
      # Service account key.
      credentials: |
        {
        "type": "service_account",
        "project_id": "project-claudie-1",
        "private_key_id": "bskdlo875s9087394763eb84e407903lskdimp439",
        "private_key": "-----BEGIN PRIVATE KEY-----\nSKLOosKJUSDANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhki\n-----END PRIVATE KEY-----\n",
        "client_email": "claudie@project-claudie-2-123456.iam.gserviceaccount.com",
        "client_id": "109876543211234567890","auth_uri": "https://accounts.google.com/o/oauth2/auth",
        "token_uri": "https://oauth2.googleapis.com/token",
        "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
        "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/claudie%40claudie-project-123456.iam.gserviceaccount.com"
        }
    - name: gcp-2
      # GCP project for the service account.
      gcpProject: project-claudie-1
      # Service account key.
      credentials: |
        {
        "type": "service_account",
        "project_id": "project-claudie-1",
        "private_key_id": "bskdlo875sregergsrh234b84e407903lskdimp439",
        "private_key": "-----BEGIN PRIVATE KEY-----\nSKLOosKJUSDANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhki\n-----END PRIVATE KEY-----\n",
        "client_email": "claudie@project-claudie-2-45y342.iam.gserviceaccount.com",
        "client_id": "4566523462523454352435","auth_uri": "https://accounts.google.com/o/oauth2/auth",
        "token_uri": "https://oauth2.googleapis.com/token",
        "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
        "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/claudie%40claudie-project-123456.iam.gserviceaccount.com"
        }

nodePools:
  dynamic:
    - name: control-gcp-1
      providerSpec:
        # Name of the provider instance.
        name: gcp-1
        # Region of the nodepool.
        region: europe-west1
        # Zone of the nodepool.
        zone: europe-west1-c
      count: 1
      # Machine type name.
      serverType: e2-medium
      # OS image name.
      image: ubuntu-os-cloud/ubuntu-2204-jammy-v20221206

    - name: control-gcp-2
      providerSpec:
        # Name of the provider instance.
        name: gcp-2
        # Region of the nodepool.
        region: europe-west1
        # Zone of the nodepool.
        zone: europe-west1-a
      count: 2
      # Machine type name.
      serverType: e2-medium
      # OS image name.
      image: ubuntu-os-cloud/ubuntu-2204-jammy-v20221206

    - name: compute-gcp-1
      providerSpec:
        # Name of the provider instance.
        name: gcp-1
        # Region of the nodepool.
        region: europe-west3
        # Zone of the nodepool.
        zone: europe-west3-a
      count: 2
      # Machine type name.
      serverType: e2-medium
      # OS image name.
      image: ubuntu-os-cloud/ubuntu-2204-jammy-v20221206
      storageDiskSize: 50

    - name: compute-gcp-2
      providerSpec:
        # Name of the provider instance.
        name: gcp-2
        # Region of the nodepool.
        region: europe-west1
        # Zone of the nodepool.
        zone: europe-west1-c
      count: 2
      # Machine type name.
      serverType: e2-medium
      # OS image name.
      image: ubuntu-os-cloud/ubuntu-2204-jammy-v20221206
      storageDiskSize: 50

kubernetes:
  clusters:
    - name: gcp-cluster
      version: v1.23.0
      network: 192.168.2.0/24
      pools:
        control:
          - control-gcp-1
          - control-gcp-2
        compute:
          - compute-gcp-1
          - compute-gcp-2
```
