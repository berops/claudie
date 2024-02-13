# Command Cheat Sheet
In this section, we'll describe `kubectl` commands to interact with Claudie.


## Monitoring the cluster state
Watch the cluster state in the `InputManifest` that is provisioned.
```json
watch -n 2 'kubectl get inputmanifests.claudie.io manifest-name -ojsonpath='{.status}' | jq .'
{
  "clusters": {
    "my-super-cluster": {
      "phase": "NONE",
      "state": "DONE"
    }
  },
  "state": "DONE"
}   
```

## Viewing the cluster metadata
  Each secret created by Claudie has following labels:

  | Key                     | Value                                           |
  | ----------------------- | ----------------------------------------------- |
  | `claudie.io/project`    | Name of the project.                            |
  | `claudie.io/cluster`    | Name of the cluster.                            |
  | `claudie.io/cluster-id` | ID of the cluster.                              |
  | `claudie.io/output`     | Output type, either `kubeconfig` or `metadata`. |

Claudie creates kubeconfig secret in claudie namespace:

  ```bash
  kubectl get secrets -n claudie -l claudie.io/output=kubeconfig
  ```
  ```
  NAME                                  TYPE     DATA   AGE
  my-super-cluster-6ktx6rb-kubeconfig   Opaque   1      134m
  ```

  You can **recover kubeconfig** for your cluster with the following command:

  ```bash
  kubectl get secrets -n claudie -l claudie.io/output=kubeconfig -o jsonpath='{.items[0].data.kubeconfig}' | base64 -d > my-super-cluster-kubeconfig.yaml
  ```

  If you want to connect to your **dynamic k8s nodes** via SSH, you can **recover private SSH** key:

  ```bash
  kubectl get secrets -n claudie -l claudie.io/output=metadata -ojsonpath='{.items[0].data.metadata}' | base64 -d | jq -r .cluster_private_key > ~/.ssh/my-super-cluster
  ```

  To **recover public IP** of your **dynamic k8s nodes** to connect to via SSH:
  ```bash
  kubectl get secrets -n claudie -l claudie.io/output=metadata -ojsonpath='{.items[0].data.metadata}' | base64 -d | jq -r .dynamic_nodepools.node_ips
  ```

  In case you want to connect to your **load balancer nodes** via SSH, you can **recover private SSH** key:

  ```bash
  kubectl get secrets -n claudie -l claudie.io/output=metadata -ojsonpath='{.items[0].data.metadata}' | base64 -d | jq -r '.load_balancer_node_pools[] | .cluster_private_key' > ~/.ssh/my-super-cluster-lb-key
  ```

  To **recover public IP** of your **load balancer** nodes to connect to via SSH:

  ```bash
  kubectl get secrets -n claudie -l claudie.io/output=metadata -ojsonpath='{.items[0].data.metadata}' | base64 -d | jq -r '.load_balancer_node_pools[] | .node_ips'
  ```
