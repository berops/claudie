package autoscaler

import (
	"testing"

	"github.com/berops/claudie/internal/templateUtils"
	"github.com/stretchr/testify/require"
)

var (
	caData = AutoscalerDeploymentData{
		ClusterID:   "test-cluster-kjbansc",
		AdapterPort: "50000",
		ClusterName: "test-cluster",
		ProjectName: "Project1",
	}

	caDeployment = `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: autoscaler-config-test-cluster-kjbansc
data:
  cloud-config: |-
    address: "autoscaler-adapter-test-cluster-kjbansc:50000"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: autoscaler-test-cluster-kjbansc
  labels:
    app: autoscaler-test-cluster-kjbansc
spec:
  replicas: 1
  selector:
    matchLabels:
      app: autoscaler-test-cluster-kjbansc
  template:
    metadata:
      labels:
        app: autoscaler-test-cluster-kjbansc
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8085"
    spec:
      containers:
        - image: registry.k8s.io/autoscaling/cluster-autoscaler:v1.25.0
          name: cluster-autoscaler
          resources:
            limits:
              cpu: 100m
              memory: 300Mi
            requests:
              cpu: 100m
              memory: 300Mi
          command:
            - ./cluster-autoscaler
            - --cloud-provider=externalgrpc
            - --cloud-config=/etc/claudie/cloud-config/cloud-config
            - --kubeconfig=/etc/claudie/kubeconfig/kubeconfig
            - --v=5
          imagePullPolicy: IfNotPresent
          volumeMounts:
            - name: kubeconfig
              mountPath: /etc/claudie/kubeconfig
              readOnly: true
            - name: cloud-config
              mountPath: /etc/claudie/cloud-config
              readOnly: true
      volumes:
        - name: kubeconfig
          secret:
            secretName: test-cluster-kjbansc-kubeconfig
        - name: cloud-config
          configMap:
            name: autoscaler-config-test-cluster-kjbansc
`
)

// TestAutoscalerTemplate tests templates generated for autoscaler.
func TestAutoscalerTemplate(t *testing.T) {
	// Load
	tpl := templateUtils.Templates{Directory: "."}
	templateLoader := templateUtils.TemplateLoader{Directory: "../../templates"}
	ca, err := templateLoader.LoadTemplate(clusterAutoscalerTemplate)
	require.NoError(t, err)
	// Check Autoscaler
	out, err := tpl.GenerateToString(ca, caData)
	require.NoError(t, err)
	require.Equal(t, out, caDeployment)
}
