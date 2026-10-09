package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/berops/claudie/proto/pb/spec"
)

func TestCreateNoProxyList(t *testing.T) {
	k8s := &spec.K8Scluster{
		ClusterInfo: &spec.ClusterInfo{
			NodePools: []*spec.NodePool{
				{Nodes: []*spec.Node{
					{Private: "192.168.2.1", Public: "1.2.3.4"},
					{Private: "192.168.2.2", Public: "1.2.3.5"},
				}},
			},
		},
	}
	lbs := []*spec.LBcluster{
		{
			ClusterInfo: &spec.ClusterInfo{
				NodePools: []*spec.NodePool{
					{Nodes: []*spec.Node{{Private: "192.168.2.3", Public: "5.6.7.8"}}},
				},
			},
			Dns: &spec.DNS{
				Endpoint: "lb.example.com",
				AlternativeNames: []*spec.AlternativeName{
					{Endpoint: "alt.example.com"},
					{Endpoint: ""},
				},
			},
		},
	}

	tests := []struct {
		name         string
		proxy        *spec.InstallationProxy
		wantContains []string
	}{
		{
			name:  "defaults",
			proxy: &spec.InstallationProxy{},
			wantContains: []string{
				"127.0.0.1/8", "localhost", "cluster.local",
				"10.244.0.0/16", "10.96.0.0/12",
				"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
				"svc", "metadata", "metadata.google.internal", "169.254.169.254", "metadata.google.internal.",
				"1.2.3.4", "1.2.3.5", "5.6.7.8",
				"lb.example.com", "alt.example.com",
			},
		},
		{
			name:  "user supplied noProxy",
			proxy: &spec.InstallationProxy{NoProxy: ".suse.com,registry.internal"},
			wantContains: []string{
				"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
				".suse.com", "registry.internal",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := createNoProxyList(k8s, lbs, tt.proxy)
			entries := strings.Split(got, ",")
			for _, want := range tt.wantContains {
				assert.Contains(t, entries, want)
			}
			// private IPs of nodes are covered by the RFC 1918 ranges and must not be listed individually.
			for _, private := range []string{"192.168.2.1", "192.168.2.2", "192.168.2.3"} {
				assert.NotContains(t, entries, private)
			}
			assert.True(t, strings.HasPrefix(got, noProxyDefault+","), "list must start with the defaults")
		})
	}
}
