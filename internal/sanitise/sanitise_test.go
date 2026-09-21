package sanitise

import (
	"testing"
)

func TestString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "converts uppercase to lowercase",
			input:    "HETZNER",
			expected: "hetzner",
		},
		{
			name:     "replaces spaces with hyphens",
			input:    "my test cluster",
			expected: "my-test-cluster",
		},
		{
			name:     "replaces colons with hyphens",
			input:    "cluster:zone",
			expected: "cluster-zone",
		},
		{
			name:     "replaces underscores with hyphens",
			input:    "nodepool_1",
			expected: "nodepool-1",
		},
		{
			name:     "replaces mixed special characters and spaces",
			input:    "My_Cluster:Zone 1",
			expected: "my-cluster-zone-1",
		},
		{
			name:     "handles consecutive spaces, colons, and underscores",
			input:    "cluster:::zone   two__three",
			expected: "cluster---zone---two--three",
		},
		{
			name:     "handles leading and trailing separators",
			input:    "__leading_and_trailing__",
			expected: "--leading-and-trailing--",
		},
		{
			name:     "preserves numbers and existing hyphens",
			input:    "k8s-cluster-123-node-456",
			expected: "k8s-cluster-123-node-456",
		},
		{
			name:     "leaves already sanitised string unchanged",
			input:    "clean-name-1",
			expected: "clean-name-1",
		},
		{
			name:     "handles empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := String(tt.input)
			if actual != tt.expected {
				t.Errorf("String(%q) = %q, want %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestURI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "masks password in mongodb connection string",
			input:    "mongodb://admin:secretPassword123@mongo.example.com:27017",
			expected: "mongodb://admin:*****@mongo.example.com:27017",
		},
		{
			name:     "masks password in http connection string",
			input:    "http://user:myPass@api.example.com",
			expected: "http://user:*****@api.example.com",
		},
		{
			name:     "masks password containing colons",
			input:    "postgres://user:my:secret:pass@localhost:5432/mydb",
			expected: "postgres://user:*****@localhost:5432/mydb",
		},
		{
			name:     "masks password when username is empty (e.g. Redis)",
			input:    "redis://:mySecretPassword@redis-server:6379",
			expected: "redis://:*****@redis-server:6379",
		},
		{
			name:     "preserves path, port, and query parameters while masking password",
			input:    "mongodb://admin:pass123@mongo.net:27017/prod_db?authSource=admin&replicaSet=rs0",
			expected: "mongodb://admin:*****@mongo.net:27017/prod_db?authSource=admin&replicaSet=rs0",
		},
		{
			name:     "leaves URI without password untouched",
			input:    "mongodb://localhost:27017",
			expected: "mongodb://localhost:27017",
		},
		{
			name:     "leaves host and port untouched",
			input:    "http://somehostname:8080",
			expected: "http://somehostname:8080",
		},
		{
			name:     "handles empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := URI(tt.input)
			if actual != tt.expected {
				t.Errorf("URI(%q) = %q, want %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestKubeconfig(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "masks single-quoted kubeconfig",
			input:    "kubectl get nodes --kubeconfig 'cluster-secret-data' -A",
			expected: "kubectl get nodes --kubeconfig '*****' -A",
		},
		{
			name:     "masks double-quoted kubeconfig",
			input:    `kubectl get nodes --kubeconfig "cluster-secret-data" -A`,
			expected: "kubectl get nodes --kubeconfig '*****' -A",
		},
		{
			name:     "masks process substitution kubeconfig",
			input:    "kubectl get pods --kubeconfig <(echo 'cluster-secret-data')",
			expected: "kubectl get pods --kubeconfig '*****'",
		},
		{
			name: "masks realistic multi-line kubeconfig with flags before and after",
			input: `kubectl apply -f app.yaml --kubeconfig 'apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: U2FsdGVkX1+...
    server: https://10.0.0.1:6443
  name: claudie-cluster
contexts:
- context:
    cluster: claudie-cluster
    user: admin
  name: default
users:
- name: admin
  user:
    client-certificate-data: U2FsdGVkX1+...' --namespace prod --wait`,
			expected: "kubectl apply -f app.yaml --kubeconfig '*****' --namespace prod --wait",
		},
		{
			name:     "leaves command without kubeconfig flag untouched",
			input:    "kubectl get pods -A",
			expected: "kubectl get pods -A",
		},
		{
			name:     "handles empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := Kubeconfig(tt.input)
			if actual != tt.expected {
				t.Errorf("Kubeconfig(%q) = %q, want %q", tt.input, actual, tt.expected)
			}
		})
	}
}
