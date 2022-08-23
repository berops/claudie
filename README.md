# Claudie - by [Berops](https://www.berops.com/)

<!-- Basic info what claudie is -->
Claudie is a simple solution for managing your multi-cloud kubernetes clusters. It is build on top of upstream tools for cluster management to simplify your needs, such as node creation, loadbalancing set up, cluster creation and more.

# Features
<!-- Why is Claudie coolest thing ever -->
### Claudie management via IaC 

Declaratively define your infrastructure with a simple, easy to understand YAML manifest.

### Fast scale-up/scale-down of your infrastructure
To scale-up or scale-down, simply change a few lines in the input manifest and Claudie will take care of the rest.

### Simple multi-cloud set up
Claudie is build with multi-cloud infrastructure in mind. Simply provide credentials to your cloud projects and watch how the infra is being build in front of you.
### Loadbalancing
TODO

### DNS management
TODO

# To get started using the Claudie

<!-- Words about possible deployments of claudie (local, in k8s cluster) -->
Claudie can be deployed
- on a local machines via docker containers (docker-compose)
- on a local machines from the go source code (Makefile)
- on a kubernetes cluster via [manifests](https://github.com/Berops/platform/tree/master/manifests/claudie)

<!-- Words about input manifest and how to input them for every deployment -->
Input
TODO

# Getting involved

<!-- Contributor guidelines -->
Everyone is more than welcome to open an issue, a PR or to start a discussion. For more information about contributing please read the [contribution guidelines](./docs/contributing/contributing.md).

# Bug reports
When you encounter a bug, please create a new [issue](https://github.com/Berops/platform/issues/new/choose) and use bug template. Before you submit, please check

- If the issue you want to open is not a duplicate
- You submitted any errors and concise way to reproduce the issue
- The input manifest you used (be careful not to include your cloud credentials) 

# Roadmap
<!-- Add a roadmap for claudie so users know which features are being worked on and which will in future -->
TODO

# License

Licensed under the Apache License, Version 2.0 [License](LICENSE)