# Deploying Claudie

## Getting Started
Claudie consists of a lot of components and it can be difficult to manage them. We recommend deploying Claudie on a small kubernetes either running locally or on cloud. You can use tools like [minikube](https://minikube.sigs.k8s.io/docs/start/) and [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) to provision a single node local cluster. 

Claudie uses [Kustomize](https://kustomize.io/) to package all required manifest defined under [`/claudie`](claudie/kustomization.yaml) and makes it easy to manage your claudie deployment. To deploy the claudie, run the following command
```bash
kubectl apply -k /claudie/kustomization.yaml
```
All the resources will be deployed under `claudie` namespace by default. 

> NOTE: Please make sure to use `manifests/claudie/kustomization.yaml` instead of `manifests/kustomization.yaml` since the later version contains [testing-framework](testing-framework/kustomization.yaml) as part of deployment. This is only used by CI for development purposes.

## For development 
To run claudie in dev, you can either follow the [getting started](#getting-started) section or run individual components in a terminal windows.

### Prerequisites 
- compatible version of [GoLang](https://go.dev/)
- [Docker Desktop](https://www.docker.com/products/docker-desktop/)
- [AWS CLI](https://aws.amazon.com/cli/)
- [golangci-ling](https://golangci-lint.run/usage/install/#local-installation) 

You can use `make` commands to run different services and configure your local environment. Here are the list of different task defined in the [Makefile](../Makefile)

```bash
Usage: make [task]

Tasks:
    gen                     Compile protobuf files to golang
    contextbox              Run context-box service 
    scheduler               Run scheduler service
    builder                 Run builder service
    terraformer             Run terraformer service
    ansibler                Run ansibler service
    kubeEleven              Run kubeEleven service
    kuber                   Run kuber service
    frontend                Run frontend service
    database                Run MongoDB container with 27017 port published
    minio                   Run Minio container with 9000 and 9001 ports published.
    dynamodb                Run DynamoDB container with 8000 port publish
    dynamodb-create-table   Create Claudie table in dynamoDB using aws cli
    dynamodb-scan-table     Print out contents of claudie table from DynamoDB using aws cli
    test                    Run testing-framework service
    dockerUp                Use docker compose to run all services
    dockerDown              Stop running services           
    dockerBuild             Build images for all services
    lint                    Run golangci-lint service
     
```