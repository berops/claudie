# Updating Claudie

In this section we'll describe how you can update resources that claudie creates based
on changes in the manifest.

## Updating Kubernetes Version

Updating the Kubernetes version is as easy as incrementing the version
in the Input Manifest of the already build cluster.

```yaml
# old version
...
kubernetes:
  clusters:
    - name: claudie-cluster
      version: v1.30.0
      network: 192.168.2.0/24
      pools:
        ...
```

```yaml
# new version
...
kubernetes:
  clusters:
    - name: claudie-cluster
      version: 1.31.0
      network: 192.168.2.0/24
      pools:
        ...
```

When re-applied this will trigger a new workflow for the cluster that will result in the updated kubernetes version.

!!! note "Downgrading a version is not supported once you've upgraded a cluster to a newer version"

# Updating Dynamic Nodepool

Nodepools specified in the InputManifest are immutable. Once created, they cannot be updated/changed. This decision was made to force the user to perform a rolling update by first deleting the nodepool and replacing it with a new version with the new desired state. A couple of examples are listed below.

## Updating the OS image

```yaml
# old version
...
- name: hetzner
  providerSpec:
    name: hetzner-1
    region: fsn1
    zone: fsn1-dc14
  count: 1
  serverType: cpx22
  image: ubuntu-22.04
...
```

```yaml
# new version
...
- name: hetzner-1 # NOTE the different name.
  providerSpec:
    name: hetzner-1
    region: fsn1
    zone: fsn1-dc14
  count: 1
  serverType: cpx22
  image: ubuntu-24.04
...
```

When re-applied this will trigger a new workflow for the cluster that will result first in the addition of the new nodepool and then the deletion of the old nodepool. 

## Changing the Server Type of a Dynamic Nodepool

The same concept applies to changing the server type of a dynamic nodepool.

```yaml
# old version
...
- name: hetzner
  providerSpec:
    name: hetzner-1
    region: fsn1
    zone: fsn1-dc14
  count: 1
  serverType: cpx22
  image: ubuntu-22.04
...
```

```yaml
# new version
...
- name: hetzner-1 # NOTE the different name.
  providerSpec:
    name: hetzner-1
    region: fsn1
    zone: fsn1-dc14
  count: 1
  serverType: cpx22
  image: ubuntu-22.04
...
```

When re-applied this will trigger a new workflow for the cluster that will result in the updated server type of the nodepool.

## Protecting Stateful Workloads with the Upgrade-Lock Label

When Claudie rolls out a new nodepool during an update, it drains the old nodes one-by-one before deleting them. For stateless workloads this is fine, but for StatefulSets with large datasets (e.g. MongoDB, PostgreSQL, Elasticsearch) replication to the newly created nodes can take longer than the 30-minute drain timeout. If Claudie force-deletes a node while replication is still in progress, the last healthy replica may be lost and data corruption can occur.

Claudie cannot determine automatically when a StatefulSet has finished replicating — every stateful workload is different and there is no universal health API. Instead, Claudie hands control over to the operator via a **node label**: any node carrying the `claudie.io/upgrade-lock` label is **skipped** during the drain phase of a rolling update.

### Workflow

1. **Before triggering the update**, mark the nodes you want to protect:
    ```bash
    kubectl label node <node-name> claudie.io/upgrade-lock=true
    ```
    The value can be anything — Claudie only checks whether the label key is present.

2. **Apply your updated InputManifest**. Claudie will:
    - Provision the new nodes
    - Drain and delete all old nodes that are **not** labeled
    - Skip the labeled nodes and log `node <name> has upgrade-lock label, skipping drain`
    - Keep the DELETE_NODES task in a retry loop, rechecking the label every ~25 seconds

3. **Verify your workload is safe to move.** Check that your StatefulSet has fully replicated data to pods on the new nodes. Use whatever tool is appropriate for your workload (e.g. `mongosh rs.status()`, `pg_stat_replication`, cluster health APIs, etc.).

4. **Remove the label when it is safe to proceed**:
    ```bash
    kubectl label node <node-name> claudie.io/upgrade-lock-
    ```
    Claudie's next retry (within ~30 seconds) will detect the removal, drain the node, and complete the rolling update.

### Important notes

- The label is a **coordination signal** only — it tells Claudie to wait. It does not affect pod scheduling on its own. If you also want to prevent new pods from landing on the locked node, apply your own `NoSchedule` taint independently.
- The label is preserved across Claudie's reconciliation cycles. You can apply it before or during an update without it being removed by Claudie.
- Claudie's retry loop runs indefinitely while the label is present — there is no timeout. Remember to remove the label when you are ready, otherwise the cluster build will never complete.
- Log evidence to look for in the kuber service:
    - `node <name> has upgrade-lock label, skipping drain` — the skip is working
    - `nodes with claudie.io/upgrade-lock label skipped, waiting for operator to remove label` — Claudie is in the retry loop
