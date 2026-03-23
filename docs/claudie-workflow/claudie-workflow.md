# Claudie

## A single platform for multiple clouds

![claudie schema](claudie-diagram.png)

### Microservices

- [Manager](https://github.com/berops/claudie/tree/master/services/manager)
- [Terraformer](https://github.com/berops/claudie/tree/master/services/terraformer)
- [Ansibler](https://github.com/berops/claudie/tree/master/services/ansibler)
- [Kube-eleven](https://github.com/berops/claudie/tree/master/services/kube-eleven)
- [Kuber](https://github.com/berops/claudie/tree/master/services/kuber)
- [Claudie-operator](https://github.com/berops/claudie/tree/master/services/claudie-operator)

### Data stores

- [MongoDB](https://github.com/berops/claudie/tree/master/manifests/claudie/mongo)
- [NATS](https://github.com/berops/claudie/tree/master/manifests/claudie/nats)
- [Minio](https://github.com/berops/claudie/tree/master/manifests/claudie/minio)

### Tools used

- [Terraform](https://github.com/hashicorp/terraform)
- [Ansible](https://github.com/ansible/ansible)
- [KubeOne](https://github.com/kubermatic/kubeone)
- [Longhorn](https://github.com/longhorn/longhorn)
- [Envoy](https://www.envoyproxy.io/)
- [Cilium](https://cilium.io/)
- [gRPC](https://grpc.io/)

## Manager

Manger is the brain and main entry point for claudie.
To build clusters users/services submit their configs to the manager service. The manager creates the desired state and schedules a number of jobs to be executed in order to achieve the desired state based on the current state. The jobs are pushed to a NATS worker queue from which the tasks are picked up by the targeted services.

For the API see the [GRPC definitions](https://github.com/berops/claudie/blob/master/proto/manager.proto).

### Flow

Each newly created manifest starts in the Pending state. Pending manifests are periodically checked for
changes from which the desired state for each cluster, along with the tasks to be performed to achieve the desired state are created,
after which the manifest is moved to the scheduled state. Tasks from Scheduled manifests are picked up
by worker services from `NATS` work queues, gradually building the desired state. From this state, the manifest can end up in either 
Done or Error state. Any changes to the input manifest while it is in the Scheduled state will be reflected after 
it is moved to the Done state. After which the cycle repeats.

Each cluster has a current state and desired state based on which tasks are created. The desired state is created only
once, when changes to the configuration are detected. Several tasks can be created that will gradually, one task at a time,
converge the current state to the desired state. Each time a task is picked up by the any of the worker services any relevant
information about what should be done is already part of that scheduled task and the service needs to simply reflect those changes
to the actual infrastructure to build/modify/delete the missing pieces.

Once a task is done building, either in error or successfully, the current state, with any changes, is communicated back to the
manager service so that it has the up to date information about the current state of the infrastructure for future tasks.

#### Rolling updates

Unless otherwise specified, the default is to use the external templates located at <https://github.com/berops/claudie-config> to build the infrastructure for the dynamic nodepools. The templates provide reasonable defaults that anyone can use to build multi-provider clusters.

As we understand that someone may need more specific scenarios, we allow these external templates to be overridden by the user, see <https://docs.claudie.io/latest/input-manifest/external-templates/> for more information.

By providing the ability to specify the templates that should be used when building the infrastructure of the InputManifest, there is one common scenario that we decided should be handled by the manager service, which is rolling updates.

Rolling updates of nodepools are performed when a change to a provider's external templates is registered. The manager then checks that the external repository of the new templates exists and uses them to perform a rolling update of the already built infrastructure.

If a failure occurs during the rolling update of a single Nodepool, the state is rolled back to the last possible working state. Rolling updates have a retry strategy that results in endless processing of rolling updates until it succeeds.

If the rollback to the last working state fails, it will also be retried indefinitely, in which case it is up to the claudie user to repair the cluster so that the rolling update can continue.

## Terraformer

Terraformer creates or destroys infrastructure via Terraform calls.

Incoming tasks are picked up from the NATS worker queue.

## Ansibler

Ansibler uses Ansible to:

- set up Wireguard VPN between the infrastructure spawned in the Terraformer service. 
- set up envoy load balancer for the infrastructure
- install dependencies for required by nodes in a kubernetes cluster

Incoming tasks are picked up from the NATS worker queue.

## Kube-eleven

Kube-eleven uses [KubeOne](https://github.com/kubermatic/kubeone) to spin up a kubernetes clusters,
out of the spawned and pre-configured infrastructure.

Incoming tasks are picked up from the NATS worker queue.

## Kuber

Kuber manipulates the cluster resources using `kubectl`.

Incoming tasks are picked up from the NATS worker queue.

## Claudie-operator

Claudie-operator is a layer between the user and Claudie. It is a `InputManifest` Custom Resource Definition controller, 
that will communicate with the `manager` service to communicate changes to the config made by the user.

### Flow

- User applies a new InputManifest crd holding a configuration of the desired clusters
- Claudie-operator detects it and processes the created/modified input manifest
- Upon deletion of user-created InputManifest, Claudie-operator initiates a deletion process of the manifest
