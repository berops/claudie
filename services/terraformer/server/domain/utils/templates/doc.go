// Package templates providers exported types and necessary helper methods
// to work with external terraform files which can be downloaded from any publicly
// available git repository.
//
// The template repository need to follow a certain convention to work properly.
// For example:
// If we consider an external template repository accessible via a public git repository at:
//
//	https://github.com/berops/claudie-config
//
// The repository may either only contain the necessary templates files or they can be stored at a
// subtree. To handle this a path is required to be passed along the public git repository such as:
//
//	templates/terraformer/gcp
//
// This denotes that the necessary templates for Google Cloud Platform can be found in the subtree at:
//
//	claudie-config/templates/terraformer/gcp
//
// To only deal with the necessary template files a sparse-checkout is used when downloading the external
// repository to have a local mirror present which will then be used to generate the terraform files.
// When using the template files for generation the subtree present at "claudie-config/templates/terraformer/gcp"
// the directory is traversed and the following rules apply:
//
//   - if a subdirectory with name "provider" is present, all files within this directory will be considered as related to
//     Providers for interacting with the API of respective Cloud Providers, SaaS providers etc. When using the templates
//     for generation, the struct [templates.Provider] will be passed for each file individually.
//
//   - if a subdirectory with name "networking" is present all files within this directory will be considered as related
//     spawning a common networking infrastructure for all nodepools from a single provider. The files in this subdirectory
//     will use the providers generated in the previous step. When using the templates the struct [templates.Networking]
//     will be passed for each file individually.
//
//   - if a subdirectory with name "nodepool" is present all files within this directory will be considered as related
//     to spawning the VM instances along with attached disk and related resources for a single node coming from a specific
//     nodepool. When using the templates the struct [templates.Nodepools] will be passed for each file individually.
//
//   - if a subdirectory with name "dns" is present, all files within this directory will be considered as related to DNS.
//     Thus, the [templates.DNS] struct will be passed for each file when generating the templates.
//     Note: This subdirectory should contain its own file that will generate the Provider needed for interacting with
//     the necessary API of the respective cloud providers (the ones that will be generated from the "provider" subdirectory
//     will not be used in this case).
//
// The complete structure of a subtree for a single provider for external templates located at claudie-config/templates/terraformer/gcp
// can look as follows:
//
//	└── terraformer
//	    |── gcp
//	    │	├── dns
//	    │   	└── dns.tpl
//	    │	├── networking
//	    │		└── networking.tpl
//	    │	├── nodepool
//	    │		├── node.tpl
//	    │		└── node_networking.tpl
//	    │	└── provider
//	    │		└── provider.tpl
//		...
//
// Examples of external templates can be found on:  https://github.com/berops/claudie-config
package templates
