package autoscaler

import (
	"testing"

	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	caData = autoscalerDeploymentData{
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
  labels:
    app.kubernetes.io/part-of: claudie
    app.kubernetes.io/name: cluster-autoscaler
    app.kubernetes.io/instance: cluster-autoscaler-test-cluster-kjbansc
    app.kubernetes.io/component: cluster-autoscaler
data:
  cloud-config: |-
    address: "localhost:50000"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: autoscaler-test-cluster-kjbansc
  labels:
    app.kubernetes.io/part-of: claudie
    app.kubernetes.io/name: cluster-autoscaler
    app.kubernetes.io/instance: cluster-autoscaler-test-cluster-kjbansc
    app.kubernetes.io/component: cluster-autoscaler
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/part-of: claudie
      app.kubernetes.io/name: cluster-autoscaler
      app.kubernetes.io/instance: cluster-autoscaler-test-cluster-kjbansc
      app.kubernetes.io/component: cluster-autoscaler
  template:
    metadata:
      labels:
        app.kubernetes.io/part-of: claudie
        app.kubernetes.io/name: cluster-autoscaler
        app.kubernetes.io/instance: cluster-autoscaler-test-cluster-kjbansc
        app.kubernetes.io/component: cluster-autoscaler
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8085"
    spec:
      securityContext:
        runAsUser: 1000
        runAsGroup: 3000
        fsGroup: 2000
      containers:
        - name: autoscaler-adapter
          imagePullPolicy: IfNotPresent
          image: ghcr.io/berops/claudie/autoscaler-adapter
          securityContext:
            allowPrivilegeEscalation: false
            privileged: false
            readOnlyRootFilesystem: true
            capabilities:
              drop:
                - all
          env:
            - name: ADAPTER_PORT
              value: "50000"
            - name: CLUSTER_NAME
              value: test-cluster
            - name: PROJECT_NAME
              value: Project1
            - name: OPERATOR_HOSTNAME
              value: ""
            - name: OPERATOR_PORT
              value: ""
          resources:
            requests:
              cpu: 80m
              memory: 100Mi
            limits:
              cpu: 160m
          ports:
            - containerPort: 50000
        - name: cluster-autoscaler
          image: registry.k8s.io/autoscaling/cluster-autoscaler:
          resources:
            requests:
              cpu: 100m
              memory: 300Mi
          command:
            - ./cluster-autoscaler
            - --cloud-provider=externalgrpc
            - --cloud-config=/etc/claudie/cloud-config/cloud-config
            - --kubeconfig=/etc/claudie/kubeconfig/kubeconfig
            - --ignore-daemonsets-utilization=true
            - --balance-similar-node-groups=true
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
	ca, err := templateUtils.LoadTemplate(ClusterAutoscalerTemplate)
	require.NoError(t, err)
	// Check Autoscaler
	out, err := tpl.GenerateToString(ca, caData)
	require.NoError(t, err)
	require.Equal(t, out, caDeployment)
}

func TestManifests(t *testing.T) {
	k8s := &spec.K8Scluster{
		ClusterInfo: &spec.ClusterInfo{
			Name: "Test",
			Hash: "Test",
		},
		Kubernetes: "v1.34.0",
	}
	manifestName := "Test"

	yamls, err := Manifests(manifestName, k8s)
	assert.Nil(t, err)

	for _, y := range yamls {
		println(y.GetKind())
	}
}
