# Local testing of Claudie

In order to speed up the development, Claudie can be run locally for initial testing purposes. However, it's important to note that running Claudie locally has limitations compared to running it in a Kubernetes cluster.

## Limitations of Claudie when running locally

### Claudie Operator/CRD testing

The Operator component as well as CRDs heavily relies on the Kubernetes cluster. However, with a little hacking, you can test them, by creating local cluster (minikube/kind/...), and exporting environment variable `KUBECONFIG` pointing to the local cluster Kubeconfig. Once you start the Claudie Operator, it should pick up the Kubeconfig and you can use local cluster to deploy and test CRDs.

### Autoscaling testing

Testing or simulating the Claudie autoscaling is not feasible when running Claudie locally because it dynamically deploys Cluster Autoscaler and Autoscaler Adapter in the management cluster.

### Claudie outputs

Since Claudie generates two types of output per cluster (node metadata and kubeconfig), testing these outputs is not possible because they are created as Kubernetes Secrets.

## Requirements to run Claudie locally

As Claudie uses number of external tools to build and manage clusters, it is important these tools are installed on your local system.

- `go` - check current version used in `go.mod` file
- `terraform` - check current version used in Terraformer Dockerfile
- `ansible` - check current version used in Ansibler Dockerfile
- `kubeone` - check current version used in Kube-eleven Dockerfile
- `kubectl` - check current version used in Kuber Dockerfile
- `mongo` - when running locally, we recommend to run `mongo` as a container, check current version used in manifests for Mongo
- `dynamo` - when running locally, we recommend to run `dynamo` as a container, check current version used in manifests for Dynamo
- `minio` - when running locally, we recommend to run `minio` as a container, check current version used in manifests for Minio

## How to run Claudie locally

To simplify the deployment of Claudie into local system, we recommend to use rules defined in Makefile.

To start all the datastores, simply run `make datastoreStart`, which will create containers for each required datastore with preconfigured port-forwarding.

To start all services, run `make <service name>`, in separate shells. In case you will make some changes to the code, to apply them, please kill the process and start it again using `make <service name>`.

## How to test Claudie locally

Once Claudie is up and running, there are three main ways to test it locally.

### Test Claudie using Testing-framework

You can test Claudie deployed locally via custom made testing framework. It was designed to support testing from local so the code itself does not require any changes. However, in order to supply testing input manifest, you have to create directory called `test-sets` in the `./testing-framework`, which will contain the input manifests. Bear in mind that these manifest are not CRDs, rather they are raw YAML file which is described in `/internal/manifest/manifest.go`.

This way of testing brings benefits like automatic verification of Longhorn deployment or automatic clean up of the infrastructure upon failure.

To run the Testing-framework locally, use `make test` rule which will start the testing. If you wish to disable the automatic clean up, set the environment variable `AUTO_CLEAN_UP` to `FALSE`.

Example of directory structure:

```sh
services/testing-framework/
├── ...
└── test-sets
    └── test-set-dev
        ├── 1.yaml
        ├── 2.yaml
        └── 3.yaml

```

Example of raw YAML input manifest:

```yaml
name: TestSetDev

providers:
  hetzner:
    - name: hetzner-1
      credentials: "api token"
  gcp:
    - name: gcp-1
      credentials: |
        service account key as JSON
      gcpProject: "project id"
  oci:
    - name: oci-1
      privateKey: |
        -----BEGIN RSA PRIVATE KEY-----
        ..... put the private key here ....
        -----END RSA PRIVATE KEY-----
      keyFingerprint: "key fingerprint"
      tenancyOcid: "tenancy ocid"
      userOcid: "user ocid"
      compartmentOcid: "compartment ocid"
  aws:
    - name: aws-1
      accessKey: "access key"
      secretKey: "secret key"
  azure:
    - name: azure-1
      subscriptionId: "subscription id"
      tenantId: "tenant id"
      clientId: "client id"
      clientSecret: "client secret"
  hetznerdns:
    - name: hetznerdns-1
      apiToken: "api token"
  cloudflare:
    - name: cloudflare-1
      apiToken: "api token"

nodePools:
  dynamic:
    - name: hetzner-control
      providerSpec:
        name: hetzner-1
        region: nbg1
        zone: nbg1-dc3
      count: 1
      serverType: cpx11
      image: ubuntu-22.04

    - name: hetzner-compute
      providerSpec:
        name: hetzner-1
        region: nbg1
        zone: nbg1-dc3
      count: 1
      serverType: cpx11
      image: ubuntu-22.04
      storageDiskSize: 50

    - name: hetzner-lb
      providerSpec:
        name: hetzner-1
        region: nbg1
        zone: nbg1-dc3
      count: 1
      serverType: cpx11
      image: ubuntu-22.04

  static:
    - name: static-pool
      nodes:
        - endpoint: "192.168.52.1"
          privateKey: |
            -----BEGIN RSA PRIVATE KEY-----
            ...... put the private key here .....
            -----END RSA PRIVATE KEY-----
        - endpoint: "192.168.52.2"
          privateKey: |
            -----BEGIN RSA PRIVATE KEY-----
            ...... put the private key here .....
            -----END RSA PRIVATE KEY-----

kubernetes:
  clusters:
    - name: dev-test
      version: v1.26.1
      network: 192.168.2.0/24
      pools:
        control:
          - static-pool
        compute:
          - hetzner-compute

loadBalancers:
  roles:
    - name: apiserver-lb
      protocol: tcp
      port: 6443
      targetPort: 6443
      targetPools: 
        - static-pool
  clusters:
    - name: miro-lb
      roles:
        - apiserver-lb
      dns:
        dnsZone: zone.com
        provider: cloudflare-1
      targetedK8s: dev-test
      pools:
        - hetzner-lb
```

### Test Claudie using manual manifest injection

To test Claudie in a more "manual" way, you can use the test client to inject an input manifest. The code for the client can be found in `services/context-box/client/client_test.go`, specifically in the `TestSaveConfigOperator` function.

In this function, the input manifest (in raw YAML format, not CRD) is located based on the `manifestFile` variable and applied to Claudie. It's important to note that this method of testing does not provide automatic clean up or verification of Longhorn deployment. Therefore, exercise caution when using this testing approach.

To trigger a deletion of the input manifest, you can use function `TestDeleteConfig` from the same test client.

### Deploy Claudie in the local cluster for testing

Claudie can be also tested on a local cluster by following these steps.

1. Spin up a local cluster using a tool like Kind, Minikube, or any other preferred method.

2. Build the images for Claudie from the current source code by running the command `make containerimgs`. This command will build all the necessary images for Claudie and assign a new tag; a short hash from the most recent commit.

3. Update the new image tag in the relevant `kustomization.yaml` files. These files can be found in the `./manifests` directory. Additionally, set the `imagePullPolicy` to `Never`.

4. Import the built images into your local cluster. This step will vary depending on the specific tool you're using for the local cluster. Refer to the documentation of the cluster tool for instructions on importing custom images.

5. Apply the Claudie manifests to the local cluster.

By following these steps, you can set up and test Claudie on a local cluster using the newly built images. Remember, these steps are going to be repeated if you will make changes to the source code.
