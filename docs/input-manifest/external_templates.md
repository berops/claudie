Claudie allows to plug in your own templates for spawning the infrastructure. Specifying which templates are to be used is done at the provider level in the Input Manifest, for example: 

```yaml
apiVersion: claudie.io/v1beta1
kind: InputManifest
metadata:
  name: genesis-example
  labels:
    app.kubernetes.io/part-of: claudie
spec:
  providers:
    - name: genesiscloud
      providerType: genesiscloud
      templates:
        repository: "https://github.com/berops/claudie-config"
        tag: "v0.9.0" # optional
        path: "templates/terraformer/genesiscloud"
      secretRef:
        name: genesiscloud-secret
        namespace: secrets
...
```

- if no templates are specified it will always default to the latest commit on the Master/Main branch of the respective cloudprovider on the berops repository (i.e. ` https://github.com/Despire/claudie-config`).

- if templates are specified, but no tag is present it will default to the latest commit of the Master/Main branch of the respective repository.

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