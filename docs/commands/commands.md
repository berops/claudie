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
  | `claudie.io/project`    | Name of the project.                          |
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
  kubectl get secrets -n claudie -l claudie.io/output=kubeconfig,claudie.io/cluster=$YOUR-CLUSTER-NAME -o jsonpath='{.items[0].data.kubeconfig}' | base64 -d > my-super-cluster-kubeconfig.yaml
  ```

  If you want to connect to your **dynamic k8s nodes** via SSH, you can **recover private SSH** key for each nodepool:

  ```bash
  kubectl get secrets -n claudie -l claudie.io/output=metadata,claudie.io/cluster=$YOUR-CLUSTER-NAME -ojsonpath='{.items[0].data.metadata}' | base64 -d | jq '.dynamic_nodepools | map_values(.nodepool_private_key)'
  ```

  To **recover public IP** of your **dynamic k8s nodes** to connect to via SSH:
  ```bash
  kubectl get secrets -n claudie -l claudie.io/output=metadata,claudie.io/cluster=$YOUR-CLUSTER-NAME -ojsonpath='{.items[0].data.metadata}' | base64 -d | jq '.dynamic_nodepools | map_values(.node_ips)'
  ```

  You can display all **dynamic load balancer nodes** metadata by:

  ```bash
  kubectl get secrets -n claudie -l claudie.io/output=metadata,claudie.io/cluster=$YOUR-CLUSTER-NAME -ojsonpath='{.items[0].data.metadata}' | base64 -d | jq -r .dynamic_load_balancer_nodepools
  ```

  In case you want to connect to your **dynamic load balancer nodes** via SSH, you can **recover private SSH** key:

  ```bash
  kubectl get secrets -n claudie -l claudie.io/output=metadata,claudie.io/cluster=$YOUR-CLUSTER-NAME -ojsonpath='{.items[0].data.metadata}' | base64 -d | jq '.dynamic_load_balancer_nodepools | .[]'
  ```

  To **recover public IP** of your **dynamic load balancer nodes** to connect to via SSH:

  ```bash
  kubectl get secrets -n claudie -l claudie.io/output=metadata,claudie.io/cluster=$YOUR-CLUSTER-NAME -ojsonpath='{.items[0].data.metadata}' | base64 -d | jq '.dynamic_load_balancer_nodepools | .[] | map_values(.node_ips)'
  ```

  You can display all **static load balancer nodes** metadata by:
  ```bash
  kubectl get secrets -n claudie -l claudie.io/output=metadata,claudie.io/cluster=$YOUR-CLUSTER-NAME -ojsonpath='{.items[0].data.metadata}' | base64 -d | jq -r .static_load_balancer_nodepools
  ```

  In order to display **public IPs** and **private SSH** keys of your **static load balancer** nodes by:

  ```bash
  kubectl get secrets -n claudie -l claudie.io/output=metadata,claudie.io/cluster=$YOUR-CLUSTER-NAME -ojsonpath='{.items[0].data.metadata}' | base64 -d | jq -r '.static_load_balancer_nodepools | .[] | map_values(.node_info)'
  ```

  To connect to one of your **static load balancer** nodes via SSH, you can **recover private SSH** key:

  ```bash
  kubectl get secrets -n claudie -l claudie.io/output=metadata,claudie.io/cluster=$YOUR-CLUSTER-NAME -ojsonpath='{.items[0].data.metadata}' | base64 -d | jq -r '.static_load_balancer_nodepools | .[]'
  ```
