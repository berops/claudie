package autoscaler

import (
	"testing"

	"github.com/berops/claudie/internal/templateUtils"
	"github.com/stretchr/testify/require"
)

var (
	aaData = AutoscalerAdapterDeploymentData{
		ClusterID:   "test-cluster-kjbansc",
		ClusterName: "test-cluster",
		ProjectName: "Project1",
		AdapterPort: "50000",
	}
	caData = AutoscalerDeploymentData{
		ClusterID:   "test-cluster-kjbansc",
		AdapterPort: "50000",
	}

	aaDeployment = `---
apiVersion: v1
kind: Service
metadata:
  name: autoscaler-adapter-test-cluster-kjbansc
spec:
  selector:
    app: autoscaler-adapter-test-cluster-kjbansc
  ports:
    - protocol: TCP
      port: 50000
      targetPort: 50000
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: autoscaler-adapter-test-cluster-kjbansc
spec:
  replicas: 1
  selector:
    matchLabels:
      app: autoscaler-adapter-test-cluster-kjbansc
  template:
    metadata:
      labels:
        app: autoscaler-adapter-test-cluster-kjbansc
    spec:
      containers:
        - name: claudie-ca
          imagePullPolicy: IfNotPresent
          image: ghcr.io/berops/claudie/autoscaler-adapter
          env:
            - name: ADAPTER_PORT
              value: "50000"
            - name: CLUSTER_NAME
              value: test-cluster
            - name: PROJECT_NAME
              value: Project1
          resources:
            requests:
              cpu: 80m
              memory: 50Mi
            limits:
              cpu: 160m
              memory: 100Mi
          ports:
            - containerPort: 50000
`
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
        - image: k8s.gcr.io/autoscaling/cluster-autoscaler:v1.25.0
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
	aa, err := templateLoader.LoadTemplate(autoscalerAdapterTemplate)
	require.NoError(t, err)
	// Check adapter
	out, err := tpl.GenerateToString(aa, aaData)
	require.NoError(t, err)
	require.Equal(t, out, aaDeployment)
	// Check Autoscaler
	out, err = tpl.GenerateToString(ca, caData)
	require.NoError(t, err)
	require.Equal(t, out, caDeployment)
}
