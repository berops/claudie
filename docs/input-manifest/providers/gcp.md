# GCP
GCP provider requires you to input multiline `credentials` as well as specific GCP project `gcpProject` where to provision resources.

## Compute and DNS example

```yaml
providers:
  gcp:
    - name: gcp-1
      gcpProject: sa-project-name
      credentials: |
        {
          "type":"service_account",
          "project_id":"project-claudie",
          "private_key_id":"private_key_id",
          "private_key":"private_key_string",
          "client_email":"claudie@project-claudie-123456.iam.gserviceaccount.com",
          "client_id":"109876543211234567890",
          "auth_uri":"https://accounts.google.com/o/oauth2/auth",
          "token_uri":"https://oauth2.googleapis.com/token",
          "auth_provider_x509_cert_url":"https://www.googleapis.com/oauth2/v1/certs",
          "client_x509_cert_url":"https://www.googleapis.com/robot/v1/metadata/x509/claudie%40claudie-project-123456.iam.gserviceaccount.com"
        }
```

## Create GCP credentials
### Prerequisites
1. Install gcoud CLI on your machine by following [this guide](https://cloud.google.com/sdk/docs/install).
2. Initialize gcloud CLI by following [this guide](https://cloud.google.com/sdk/docs/initializing).
3. Authorize cloud CLI by following [this guide](https://cloud.google.com/sdk/docs/authorizing)

### Creating GCP credentials for Claudie

1. Create a GCP project:
```bash
gcloud projects create claudie-project
```

2. Set the current project to claudie-project:
```bash
gcloud config set project claudie-project
```

3. Attach billing account to your project:
```bash
gcloud alpha billing accounts projects link claudie-project (--account-id=ACCOUNT_ID | --billing-account=ACCOUNT_ID)
```

4. Enable Compute Engine API and Cloud DNS API:
```bash
{
  gcloud services enable compute.googleapis.com
  gcloud services enable dns.googleapis.com
}
```

5. Create a service account:
```bash
gcloud iam service-accounts create claudie-sa
```

6. Attach roles to the servcie account:
```bash
{
  gcloud projects add-iam-policy-binding claudie-project --member=serviceAccount:claudie-sa@claudie-project.iam.gserviceaccount.com --role=roles/compute.admin
  gcloud projects add-iam-policy-binding claudie-project --member=serviceAccount:claudie-sa@claudie-project.iam.gserviceaccount.com --role=roles/dns.admin
}
```

7. Recover service account keys for claudie-sa:
```bash
gcloud iam service-accounts keys create claudie.json --iam-account=claudie-sa@claudie-project.iam.gserviceaccount.com
```

8. You can either manually perform this step or use `yq` with the [provided template](#compute-and-dns-example) to safely replace the credentials field with your generated credentials:
```bash
yq '.providers.gcp[0].credentials = load_str("claudie.json")' template.yaml > gcp.yaml
```

## DNS setup
If you wish to use GCP as your DNS provider where Claudie creates DNS records pointing to Claudie managed clusters, you will need to create a **public DNS zone** by following [this guide](https://cloud.google.com/dns/docs/zones).

!!! warning "GCP is not my domain registrar"
    If you haven't acquired a domain via GCP and wish to utilize GCP for hosting your zone, you can refer to [this guide](https://cloud.google.com/dns/docs/update-name-servers) on GCP nameservers. However, if you prefer not to use the entire domain, an alternative option is to delegate a subdomain to GCP.

## Input manifest examples
### Single provider, multi region cluster example

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
         "type":"service_account",
         "project_id":"project-claudie",
         "private_key_id":"bskdlo875s9087394763eb84e407903lskdimp439",
         "private_key":"-----BEGIN PRIVATE KEY-----\nSKLOosKJUSDANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCj2/askJSLosad\nMIIEvQIBADANBgkqhki\n-----END PRIVATE KEY-----\n",
         "client_email":"claudie@project-claudie-123456.iam.gserviceaccount.com",
         "client_id":"109876543211234567890",
         "auth_uri":"https://accounts.google.com/o/oauth2/auth",
         "token_uri":"https://oauth2.googleapis.com/token",
         "auth_provider_x509_cert_url":"https://www.googleapis.com/oauth2/v1/certs",
         "client_x509_cert_url":"https://www.googleapis.com/robot/v1/metadata/x509/claudie%40claudie-project-123456.iam.gserviceaccount.com"
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
      version: v1.24.0
      network: 192.168.2.0/24
      pools:
        control:
          - control-gcp
        compute:
          - compute-1-gcp
          - compute-2-gcp
```

### Multi provider, multi region clusters example

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
      version: v1.24.0
      network: 192.168.2.0/24
      pools:
        control:
          - control-gcp-1
          - control-gcp-2
        compute:
          - compute-gcp-1
          - compute-gcp-2
```
