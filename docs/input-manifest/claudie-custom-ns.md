# Deploying Claudie in a custom namespace

By default, when following the [Getting Started](../getting-started/get-started-using-claudie.md#install-claudie) guide, Claudie is deployed in the `claudie` namespace. However, you may want to deploy it into a custom namespace for reasons such as organizational structure, environment isolation or others.

## Modifiyng claudie.yaml bundle

1. Download the latest claudie.yaml 
    ```bash
    wget https://github.com/berops/claudie/releases/latest/download/claudie.yaml
    ```
2. Before applying the manifest, make the following changes:
   
    2.1. Replace every occurrence of `namespace: claudie` with your desired namespace (e.g., new-namespace). 
   Using linux terminal you can use sed utility:
   ```bash
    sed -i 's/namespace: claudie/namespace: new-namespace/' claudie.yaml
   ```
    2.2. For DNS Names within Certificate resource, `kind: Certificate`, ensure the dnsNames reflect the new namespace:
   ```yaml
   spec:
       dnsNames:
       - claudie-operator.new-namespace
       - claudie-operator.new-namespace.svc
       - claudie-operator.new-namespace.svc.cluster
       - claudie-operator.new-namespace.svc.cluster.local
   ```
   Using linux terminal you can use sed utility:
   ```bash
   sed -i 's/\(claudie-operator\)\.claudie/\1.new-namespace/g' claudie.yaml
   ```
   2.3. Replace annotations `cert-manager.io/inject-ca-from: claudie/claudie-webhook-certificate` and name `name: claudie-webhook` in ValidatingWebhookConfiguration resource, `kind: ValidatingWebhookConfiguration`, so that is contains name of your new namespace
    ```yaml
    annotations:
        cert-manager.io/inject-ca-from: new-namespace/claudie-webhook-certificate
    ...
    name: claudie-webhook-new-namespace
    ```
    Using linux terminal you can use sed utility:

    ```bash
    sed -i 's/cert-manager\.io\/inject-ca-from: claudie\//cert-manager.io\/inject-ca-from: new-namespace\//g' claudie.yaml
    sed -i 's/claudie-webhook$/claudie-webhook-new-namespace/g' claudie.yaml
    ```
     2.4. To restrict the namespaces monitored by the Claudie operator (as defined in `claudie.yaml`), add the `CLAUDIE_NAMESPACES` environment variable to the claudie-operator deployment.
     ```yaml
     env:
        - name: CLAUDIE_NAMESPACES
          value: "new-namespace"
     ```
     2.5. To ensure the `ClusterRoleBinding` is correctly applied to the specified `ServiceAccount`, make sure the `ClusterRoleBinding` has a unique name. Modify the name of the `ClusterRoleBinding` resource in the `claudie.yaml`.
     
     Using linux terminal you can use sed utility:

     ```bash
     sed -i 's/claudie-operator-role-binding/claudie-operator-role-binding-new-namespace/g' claudie.yaml
     ```
     2.6. Once you’ve created claudie.yaml, create your custom namespace and apply the manifest. Make sure Cert Manager is already deployed in your cluster
    ```bash
    kubectl create namespace new-namespace
    kubectl apply -f claudie.yaml -n brando
    ```