# GCP
GCP provider requires you to input multiline `credentials` as well as specific GCP project ID `gcpproject` where to provision resources.

## Compute and DNS example

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gcp-secret
data:
  credentials: <base64-encoded-service-account-json>
  gcpproject: <base64-encoded-project-id>
type: Opaque
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
gcloud alpha billing accounts projects link claudie-project (--account-id=<billing-account-id> | --billing-account=<billing-account-id>)
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
gcloud iam service-accounts keys create claudie-credentials.json --iam-account=claudie-sa@claudie-project.iam.gserviceaccount.com
```

## DNS setup
If you wish to use GCP as your DNS provider where Claudie creates DNS records pointing to Claudie managed clusters, you will need to create a **public DNS zone** by following [this guide](https://cloud.google.com/dns/docs/zones).

!!! warning "GCP is not my domain registrar"
    If you haven't acquired a domain via GCP and wish to utilize GCP for hosting your zone, you can refer to [this guide](https://cloud.google.com/dns/docs/update-name-servers) on GCP nameservers. However, if you prefer not to use the entire domain, an alternative option is to delegate a subdomain to GCP.

## Input manifest examples
### Single provider, multi region cluster example

### Create a secret for Cloudflare and GCP providers
The secret for an GCP provider must include the following mandatory fields: `gcpproject` and `credentials`.
```bash
# The ./claudie-credentials.json file is the file created in #Creating GCP credentials for Claudie step 7.
kubectl create secret generic gcp-secret-1 --namespace=<your-namespace> --from-literal=gcpproject='<your-project-id>' --from-file=credentials=./claudie-credentials.json
```

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: gcp-example-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: gcp-1
      providerType: gcp
      secretRef:
        name: gcp-secret-1
        namespace: <your-namespace>

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
        image: ubuntu-2404-noble-amd64-v20251001

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
        image: ubuntu-2404-noble-amd64-v20251001
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
        image: ubuntu-2404-noble-amd64-v20251001
        storageDiskSize: 50

  kubernetes:
    clusters:
      - name: gcp-cluster
        version: v1.31.0
        network: 192.168.2.0/24
        pools:
          control:
            - control-gcp
          compute:
            - compute-1-gcp
            - compute-2-gcp
```

### Multi provider, multi region clusters example

### Create a secret for Cloudflare and GCP providers
The secret for an GCP provider must include the following mandatory fields: `gcpproject` and `credentials`.
```bash
# The ./claudie-credentials.json file is the file created in #Creating GCP credentials for Claudie step 7.
kubectl create secret generic gcp-secret-1 --namespace=<your-namespace> --from-literal=gcpproject='<your-project-id>' --from-file=credentials=./claudie-credentials.json
kubectl create secret generic gcp-secret-2 --namespace=<your-namespace> --from-literal=gcpproject='<your-project-id>' --from-file=credentials=./claudie-credentials-2.json
```

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: gcp-example-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: gcp-1
      providerType: gcp
      secretRef:
        name: gcp-secret-1
        namespace: <your-namespace>
    - name: gcp-2
      providerType: gcp
      secretRef:
        name: gcp-secret-2
        namespace: <your-namespace>

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
        image: ubuntu-2404-noble-amd64-v20250313

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
        image: ubuntu-2404-noble-amd64-v20250313

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
        image: ubuntu-2404-noble-amd64-v20250313
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
        image: ubuntu-2404-noble-amd64-v20250313
        storageDiskSize: 50

  kubernetes:
    clusters:
      - name: gcp-cluster
        version: v1.31.0
        network: 192.168.2.0/24
        pools:
          control:
            - control-gcp-1
            - control-gcp-2
          compute:
            - compute-gcp-1
            - compute-gcp-2
```
