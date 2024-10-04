package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	"google.golang.org/protobuf/proto"
)

func Test_rollingUpdateLB(t *testing.T) {
	rngHash := utils.CreateHash(utils.HashLength)
	np := &spec.NodePool{
		Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
			ServerType:      "type",
			Image:           "image",
			StorageDiskSize: 50,
			Region:          "local",
			Zone:            "earth",
			Count:           1,
			Provider: &spec.Provider{
				SpecName:          "local-provider",
				CloudProviderName: "local",
				ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
				Templates:         &spec.TemplateRepository{Repository: "https://github.com/berops/claudie", Path: "path-1", CommitHash: "hash-1"},
			},
			PublicKey:  "pk",
			PrivateKey: "sk",
			Cidr:       "cidr",
		}},
		Name: fmt.Sprintf("np-%s", rngHash),
		Nodes: []*spec.Node{
			{Name: "node-0", Private: "private", Public: "public", NodeType: spec.NodeType_apiEndpoint, Username: "root"},
		},
		IsControl: true,
	}
	np2 := &spec.NodePool{
		Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
			ServerType:      "type",
			Image:           "image",
			StorageDiskSize: 50,
			Region:          "local",
			Zone:            "earth",
			Count:           3,
			Provider: &spec.Provider{
				SpecName:          "local-provider",
				CloudProviderName: "local",
				ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
				Templates:         &spec.TemplateRepository{Repository: "https://github.com/berops/claudie", Path: "path-1", CommitHash: "hash-1"},
			},
			PublicKey:  "pk",
			PrivateKey: "sk",
			Cidr:       "cidr",
		}},
		Name: fmt.Sprintf("np2-%s", rngHash),
		Nodes: []*spec.Node{
			{Name: "node-0", Private: "private-1", Public: "public-1", NodeType: spec.NodeType_worker, Username: "root"},
			{Name: "node-1", Private: "private-2", Public: "public-2", NodeType: spec.NodeType_worker, Username: "root"},
			{Name: "node-2", Private: "private-3", Public: "public-3", NodeType: spec.NodeType_worker, Username: "root"},
		},
		IsControl: false,
	}
	np3 := &spec.NodePool{
		Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
			ServerType:      "type",
			Image:           "image",
			StorageDiskSize: 50,
			Region:          "local",
			Zone:            "earth",
			Count:           3,
			Provider: &spec.Provider{
				SpecName:          "local-provider",
				CloudProviderName: "local",
				ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
				Templates:         &spec.TemplateRepository{Repository: "https://github.com/berops/claudie", Path: "path-1", CommitHash: "hash-1"},
			},
			PublicKey:  "pk",
			PrivateKey: "sk",
			Cidr:       "cidr",
		}},
		Name: fmt.Sprintf("np3-%s", rngHash),
		Nodes: []*spec.Node{
			{Name: "node-0", Private: "private-1", Public: "public-1", NodeType: spec.NodeType_worker, Username: "root"},
			{Name: "node-0", Private: "private-1", Public: "public-1", NodeType: spec.NodeType_worker, Username: "root"},
			{Name: "node-0", Private: "private-1", Public: "public-1", NodeType: spec.NodeType_worker, Username: "root"},
		},
		IsControl: false,
	}

	current := &spec.Clusters{
		K8S: &spec.K8Scluster{
			ClusterInfo: &spec.ClusterInfo{
				Name:      "cluster",
				Hash:      "hash",
				NodePools: []*spec.NodePool{proto.Clone(np).(*spec.NodePool)},
			},
		},
		LoadBalancers: &spec.LoadBalancers{
			Clusters: []*spec.LBcluster{
				{
					ClusterInfo: &spec.ClusterInfo{
						Name: "lb-cluster",
						Hash: "hash",
						NodePools: []*spec.NodePool{
							proto.Clone(np2).(*spec.NodePool),
							proto.Clone(np3).(*spec.NodePool),
						},
					},
					TargetedK8S: "cluster",
				},
			},
		},
	}

	type args struct {
		current  *spec.Clusters
		desired  *spec.Clusters
		position int
	}
	tests := []struct {
		name       string
		args       args
		want       *spec.LoadBalancers
		wantEvents []*spec.TaskEvent
		wantErr    assert.ErrorAssertionFunc
		validate   func(t *testing.T, args args, got *spec.LoadBalancers)
	}{
		{
			name: "ok-no-rolling-update",
			args: args{
				current:  proto.Clone(current).(*spec.Clusters),
				desired:  proto.Clone(current).(*spec.Clusters),
				position: 0,
			},
			want:       proto.Clone(current.LoadBalancers).(*spec.LoadBalancers),
			wantEvents: nil,
			wantErr:    func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
			validate: func(t *testing.T, args args, got *spec.LoadBalancers) {
				if !assert.True(t, proto.Equal(current.LoadBalancers, got)) {
					diff := cmp.Diff(current.LoadBalancers, got, opts)
					t.Fatalf("%s", diff)
				}
			},
		},
		{
			name: "ok-rolling-update-np2",
			args: args{
				current: proto.Clone(current).(*spec.Clusters),
				desired: func() *spec.Clusters {
					desired := proto.Clone(current).(*spec.Clusters)
					desired.LoadBalancers.Clusters[0].ClusterInfo.NodePools[0].GetDynamicNodePool().Provider.Templates.CommitHash = "hash-3"
					return desired
				}(),
				position: 0,
			},
			want: proto.Clone(current.LoadBalancers).(*spec.LoadBalancers),
			wantEvents: []*spec.TaskEvent{
				{Event: spec.Event_UPDATE, Description: fmt.Sprintf("rolling update loadbalancers: replacing np2-%s with ", rngHash)},
				{Event: spec.Event_UPDATE, Description: fmt.Sprintf("rolling update lbs: deleting infrastructure of deleted nodes from nodepool np2-%s", rngHash)},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
			validate: func(t *testing.T, args args, got *spec.LoadBalancers) {
				assert.True(t, !proto.Equal(args.current.LoadBalancers, got))
				assert.True(t, !proto.Equal(args.desired.LoadBalancers, got))

				assert.Equal(t, 2, len(got.Clusters[0].ClusterInfo.NodePools))
				assert.NotEqual(t, np2.Name, got.Clusters[0].ClusterInfo.NodePools[0].Name)
				assert.NotEqual(t, np2.Name, got.Clusters[0].ClusterInfo.NodePools[1].Name)

				assert.Equal(t, np3.Name, got.Clusters[0].ClusterInfo.NodePools[1].Name)

				assert.Equal(t, np3.GetDynamicNodePool().Count, got.Clusters[0].ClusterInfo.NodePools[0].GetDynamicNodePool().Count)
				assert.Equal(t, np2.GetDynamicNodePool().Count, got.Clusters[0].ClusterInfo.NodePools[1].GetDynamicNodePool().Count)

				assert.Equal(t, spec.NodeType_worker, got.Clusters[0].ClusterInfo.NodePools[0].Nodes[0].NodeType)
				assert.Equal(t, spec.NodeType_worker, got.Clusters[0].ClusterInfo.NodePools[0].Nodes[1].NodeType)
				assert.Equal(t, spec.NodeType_worker, got.Clusters[0].ClusterInfo.NodePools[0].Nodes[2].NodeType)

				assert.NotEqual(t, "node-0", got.Clusters[0].ClusterInfo.NodePools[0].Nodes[0].Name)
				assert.NotEqual(t, "node-0", got.Clusters[0].ClusterInfo.NodePools[0].Nodes[1].Name)
				assert.NotEqual(t, "node-0", got.Clusters[0].ClusterInfo.NodePools[0].Nodes[2].Name)

				assert.NotEqual(t, np2.GetDynamicNodePool().PrivateKey, got.Clusters[0].ClusterInfo.NodePools[0].GetDynamicNodePool().PrivateKey)
				assert.Equal(t, np3.GetDynamicNodePool().PublicKey, got.Clusters[0].ClusterInfo.NodePools[1].GetDynamicNodePool().PublicKey)
			},
		},
		{
			name: "ok-rolling-update-np2-np3",
			args: args{
				current: proto.Clone(current).(*spec.Clusters),
				desired: func() *spec.Clusters {
					desired := proto.Clone(current).(*spec.Clusters)
					desired.LoadBalancers.Clusters[0].ClusterInfo.NodePools[0].GetDynamicNodePool().Provider.Templates.CommitHash = "hash-4"
					desired.LoadBalancers.Clusters[0].ClusterInfo.NodePools[1].GetDynamicNodePool().Provider.Templates.CommitHash = "hash-5"
					return desired
				}(),
				position: 0,
			},
			want: proto.Clone(current.LoadBalancers).(*spec.LoadBalancers),
			wantEvents: []*spec.TaskEvent{
				{Event: spec.Event_UPDATE, Description: fmt.Sprintf("rolling update loadbalancers: replacing np2-%s with ", rngHash)},
				{Event: spec.Event_UPDATE, Description: fmt.Sprintf("rolling update lbs: deleting infrastructure of deleted nodes from nodepool np2-%s", rngHash)},
				{Event: spec.Event_UPDATE, Description: fmt.Sprintf("rolling update loadbalancers: replacing np3-%s with ", rngHash)},
				{Event: spec.Event_UPDATE, Description: fmt.Sprintf("rolling update lbs: deleting infrastructure of deleted nodes from nodepool np3-%s", rngHash)},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
			validate: func(t *testing.T, args args, got *spec.LoadBalancers) {
				assert.True(t, !proto.Equal(args.current.LoadBalancers, got))
				assert.True(t, !proto.Equal(args.desired.LoadBalancers, got))

				assert.Equal(t, 2, len(got.Clusters[0].ClusterInfo.NodePools))
				assert.NotEqual(t, np2.Name, got.Clusters[0].ClusterInfo.NodePools[0].Name)
				assert.NotEqual(t, np2.Name, got.Clusters[0].ClusterInfo.NodePools[1].Name)

				assert.NotEqual(t, np3.Name, got.Clusters[0].ClusterInfo.NodePools[0].Name)
				assert.NotEqual(t, np3.Name, got.Clusters[0].ClusterInfo.NodePools[1].Name)

				assert.Equal(t, np3.GetDynamicNodePool().Count, got.Clusters[0].ClusterInfo.NodePools[0].GetDynamicNodePool().Count)
				assert.Equal(t, np2.GetDynamicNodePool().Count, got.Clusters[0].ClusterInfo.NodePools[1].GetDynamicNodePool().Count)

				assert.Equal(t, spec.NodeType_worker, got.Clusters[0].ClusterInfo.NodePools[0].Nodes[0].NodeType)
				assert.Equal(t, spec.NodeType_worker, got.Clusters[0].ClusterInfo.NodePools[0].Nodes[1].NodeType)
				assert.Equal(t, spec.NodeType_worker, got.Clusters[0].ClusterInfo.NodePools[0].Nodes[2].NodeType)

				assert.Equal(t, spec.NodeType_worker, got.Clusters[0].ClusterInfo.NodePools[1].Nodes[0].NodeType)
				assert.Equal(t, spec.NodeType_worker, got.Clusters[0].ClusterInfo.NodePools[1].Nodes[1].NodeType)
				assert.Equal(t, spec.NodeType_worker, got.Clusters[0].ClusterInfo.NodePools[1].Nodes[2].NodeType)

				assert.NotEqual(t, np2.Nodes[0].Name, got.Clusters[0].ClusterInfo.NodePools[0].Nodes[0].Name)
				assert.NotEqual(t, np2.Nodes[1].Name, got.Clusters[0].ClusterInfo.NodePools[0].Nodes[1].Name)
				assert.NotEqual(t, np2.Nodes[2].Name, got.Clusters[0].ClusterInfo.NodePools[0].Nodes[2].Name)

				assert.NotEqual(t, np3.Nodes[0], got.Clusters[0].ClusterInfo.NodePools[1].Nodes[0].Name)
				assert.NotEqual(t, np3.Nodes[1], got.Clusters[0].ClusterInfo.NodePools[1].Nodes[1].Name)
				assert.NotEqual(t, np3.Nodes[2], got.Clusters[0].ClusterInfo.NodePools[1].Nodes[2].Name)

				assert.NotEqual(t, np2.GetDynamicNodePool().PrivateKey, got.Clusters[0].ClusterInfo.NodePools[0].GetDynamicNodePool().PrivateKey)
				assert.NotEqual(t, np3.GetDynamicNodePool().PublicKey, got.Clusters[0].ClusterInfo.NodePools[1].GetDynamicNodePool().PublicKey)

				assert.True(t, strings.HasPrefix(got.Clusters[0].ClusterInfo.NodePools[0].Name, "np2"))
				assert.True(t, strings.HasPrefix(got.Clusters[0].ClusterInfo.NodePools[1].Name, "np3"))

				assert.Empty(t, got.Clusters[0].ClusterInfo.NodePools[0].GetDynamicNodePool().Cidr)
				assert.Empty(t, got.Clusters[0].ClusterInfo.NodePools[1].GetDynamicNodePool().Cidr)

				assert.NotEqual(t, current.LoadBalancers.Clusters[0].ClusterInfo.NodePools[0].GetDynamicNodePool().Provider.Templates.CommitHash, got.Clusters[0].ClusterInfo.NodePools[0].GetDynamicNodePool().Provider.Templates.CommitHash)

				assert.NotEqual(t, current.LoadBalancers.Clusters[0].ClusterInfo.NodePools[1].GetDynamicNodePool().Provider.Templates.CommitHash, got.Clusters[0].ClusterInfo.NodePools[1].GetDynamicNodePool().Provider.Templates.CommitHash)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotLb, got, err := rollingUpdateLB(tt.args.current, tt.args.desired, tt.args.position)
			tt.wantErr(t, err)

			assert.Equal(t, len(tt.wantEvents), len(got))
			for i := range got {
				assert.Equal(t, tt.wantEvents[i].Event, got[i].Event)

				// since we don't know the hash check for prefix
				if !assert.True(t, strings.HasPrefix(got[i].Description, tt.wantEvents[i].Description)) {
					assert.Equal(t, tt.wantEvents[i].Description, got[i].Description)
				}
			}

			tt.validate(t, tt.args, gotLb.LoadBalancers)
		})
	}
}
