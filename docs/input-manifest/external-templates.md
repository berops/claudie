Claudie allows you to plug in your own templates for spawning the infrastructure. Specifying which templates are to be used is done by creating a `TemplateGitReference` custom resource and referencing it at the provider level in the Input Manifest, for example:

```yaml
apiVersion: claudie.io/v1beta1
kind: TemplateGitReference
metadata:
  name: my-templates
  namespace: claudie
spec:
  endpoint:
    url: github.com/berops/claudie-config
    protocol: https
  commit: v0.9.19
  paths:
    terraformer: templates/terraformer
    playbooks: templates/playbooks
    configLb: templates/config-lb
    configK8s: templates/config-k8s
    manifestsK8s: templates/manifests-k8s
---
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: hetzner-example
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: hetzner-1
      providerType: hetzner
      templatesRef:
        name: my-templates
        namespace: claudie
      secretRef:
        name: hetzner-secret
        namespace: secrets
...
```

Claudie ships with a default `TemplateGitReference` named `claudie-default-templates` in the `claudie` namespace. If no `templatesRef` is specified on a provider, the default is used.

A `TemplateGitReference` references a git repository, a commit (tag, branch, or hash), and paths to template directories within that repository. Each path points to a subdirectory containing templates for a specific service (you can also read more about this CRD inside the [api-reference](./api-reference.md)):

- `terraformer` — OpenTofu infrastructure templates
- `playbooks` — Ansible playbooks
- `configLb` — Loadbalancer configuration templates
- `configK8s` — Kubernetes configuration templates
- `manifestsK8s` — Kubernetes manifest templates

The template **repository** need to follow a certain convention to work properly.
For example:
If we consider an external template repository accessible via a public git repository at:

	https://github.com/berops/claudie-config

The repository can either contain only the necessary template files, or they can be stored in a subtree. To handle this, you need to pass a **path** within the public git repository, such as

	templates/terraformer/gcp

This denotes that the necessary templates for Google Cloud Platform can be found in the subtree at:

	claudie-config/templates/terraformer/gcp

To only deal with the necessary template files a sparse-checkout is used when downloading the external
repository to have a local mirror present which will then be used to generate the terraform files.
When using the template files for generation the subtree present at the above given example `claudie-config/templates/terraformer/gcp`
the directory is traversed and the following rules apply:

- if a subdirectory with name "provider" is present, all files within this directory will be considered as related to
  Providers for interacting with the API of respective Cloud Providers, SaaS providers etc. When using the templates
  for generation, the struct [templates.Provider](https://github.com/berops/claudie/blob/5dc0e7c8f5503a6f2c202a982f5c4aa11bed0346/services/terraformer/server/domain/utils/templates/structures.go#L54) will be passed for each file individually.

- if a subdirectory with name "networking" is present all files within this directory will be considered as related
  spawning a common networking infrastructure for all nodepools from a single provider. The files in this subdirectory
  will use the providers generated in the previous step. When using the templates the struct [templates.Networking](https://github.com/berops/claudie/blob/5dc0e7c8f5503a6f2c202a982f5c4aa11bed0346/services/terraformer/server/domain/utils/templates/structures.go#L92)
  will be passed for each file individually.

- if a subdirectory with name "nodepool" is present all files within this directory will be considered as related
  to spawning the VM instances along with attached disk and related resources for a single node coming from a specific
  nodepool. When using the templates the struct [templates.Nodepools](https://github.com/berops/claudie/blob/5dc0e7c8f5503a6f2c202a982f5c4aa11bed0346/services/terraformer/server/domain/utils/templates/structures.go#L138) will be passed for each file individually.

- if a subdirectory with name "dns" is present, all files within this directory will be considered as related to DNS.
  Thus, the [templates.DNS](https://github.com/berops/claudie/blob/5dc0e7c8f5503a6f2c202a982f5c4aa11bed0346/services/terraformer/server/domain/utils/templates/structures.go#L151) struct will be passed for each file when generating the templates.
  Note: This subdirectory should contain its own file that will generate the Provider needed for interacting with
  the necessary API of the respective cloud providers (the ones that will be generated from the "provider" subdirectory
  will not be used in this case).

The complete structure of a subtree for a single provider for external templates located at claudie-config/templates/terraformer/gcp
can look as follows:

	└── terraformer
	    |── gcp
	    │	├── dns
	    │   	└── dns.tpl
	    │	├── networking
	    │		└── networking.tpl
	    │	├── nodepool
	    │		├── node.tpl
	    │		└── node_networking.tpl
	    │	└── provider
	    │		└── provider.tpl
		...

Examples of external templates can be found on:  https://github.com/berops/claudie-config

## Rolling update

To handle more specific scenarios where the default templates provided by claudie do not fit the use case, we allow these external templates to be changed/adapted by the user.

By providing this ability to specify the templates to be used when building the InputManifest infrastructure, there is one common scenario that should be handled by claudie, which is rolling updates.

Rolling updates of nodepools are performed when a change to a provider's external templates is registered. Claudie checks that the referenced `TemplateGitReference` custom resource exists and uses it to perform a rolling update of the infrastructure already built. In the below example, when the `templatesRef` of provider Hetzner-1 is changed the rolling update of all the nodepools which reference that provider will start by doing an update on a single nodepool at a time.

```diff
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: hetzner-example-manifest
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: hetzner-1
      providerType: hetzner
-     templatesRef:
-       name: claudie-default-templates
-       namespace: claudie
+     templatesRef:
+       name: my-templates
+       namespace: claudie
      secretRef:
        name: hetzner-secret-1
        namespace: <your-namespace>

  nodePools:
    dynamic:
      - name: control-htz
        providerSpec:
          # Name of the provider instance.
          name: hetzner-1
          # Location of the nodepool.
          region: hel1
        count: 1
        # Machine type name.
        serverType: cpx22
        # OS image name.
        image: ubuntu-22.04

      - name: compute-1-htz
        providerSpec:
          # Name of the provider instance.
          name: hetzner-1
          # Location of the nodepool.
          region: fsn1
        count: 2
        # Machine type name.
        serverType: cpx22
        # OS image name.
        image: ubuntu-22.04
        storageDiskSize: 50

      - name: compute-2-htz
        providerSpec:
          # Name of the provider instance.
          name: hetzner-1
          # Location of the nodepool.
          region: nbg1
        count: 2
        # Machine type name.
        serverType: cpx22
        # OS image name.
        image: ubuntu-22.04
        storageDiskSize: 50

  kubernetes:
    clusters:
      - name: hetzner-cluster
        version: v1.34.0
        network: 192.168.2.0/24
        pools:
          control:
            - control-htz
          compute:
            - compute-1-htz
            - compute-2-htz
```

The rolling update is also triggered if only the commit reference of the `TemplateGitReference` is changed.

```diff
apiVersion: claudie.io/v1beta1
kind: TemplateGitReference
metadata:
  name: my-templates
  namespace: claudie
spec:
  endpoint:
    url: github.com/berops/claudie-config
    protocol: https
- commit: v0.9.19
+ commit: v0.9.9
  paths:
    terraformer: templates/terraformer
    playbooks: templates/playbooks
    configLb: templates/config-lb
    configK8s: templates/config-k8s
    manifestsK8s: templates/manifests-k8s
```
