# Prometheus Monitoring

In our environment, we rely on Claudie to export Prometheus metrics, providing valuable insights into the state of our infrastructure and applications. To utilize Claudie's monitoring capabilities, it's essential to have Prometheus installed. With this setup, you can gain visibility into various metrics such as:

  - Number of managed K8s clusters created by Claudie
  - Number of managed LoadBalancer clusters created by Claudie
  - Currently added/deleted nodes to/from K8s/LB cluster
  - Information about gRPC requests
  - and much more

You can find [Claudie dashboard](https://grafana.com/grafana/dashboards/20064-claudie-dashboard/) here.

## Configure scraping metrics
We recommend using the [Prometheus Operator](https://github.com/prometheus-operator/kube-prometheus) for managing Prometheus deployments efficiently.

!!! note "If you are not able to access the Prometheus metrics exported by Claudie, double check you network policies"


1. Create `RBAC` that allows Prometheus to scrape metrics from Claudie’s pods:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: claudie-pod-reader
  namespace: claudie
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: claudie-pod-reader-binding
  namespace: claudie
subjects:
# this SA is created by https://github.com/prometheus-operator/kube-prometheus
# in your case you might need to bind this Role to a different SA
- kind: ServiceAccount
  name: prometheus-k8s
  namespace: monitoring
roleRef:
  kind: Role
  name: claudie-pod-reader
  apiGroup: rbac.authorization.k8s.io
```

2. Create Prometheus PodMonitor to scrape metrics from Claudie’s pods
```yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: claudie-metrics
  namespace: monitoring
  labels:
    name: claudie-metrics
spec:
  namespaceSelector:
    matchNames:
      - claudie
  selector:
    matchLabels:
      app.kubernetes.io/part-of: claudie
  podMetricsEndpoints:
  - port: metrics
```
3. Import [our dashboard](https://grafana.com/grafana/dashboards/20064-claudie-dashboard/) into your Grafana instance:
    * Navigate to the Grafana UI.
    * Go to the Dashboards section.
    * Click on "Import" and provide the dashboard ID or upload the JSON file.
    * Configure the data source to point to your Prometheus instance.
    * Save the dashboard, and you're ready to visualize Claudie's metrics in Grafana.

That's it! Now you have set up RBAC for Prometheus, configured a PodMonitor to scrape metrics from Claudie's pods, and imported a Grafana dashboard to visualize the metrics.
