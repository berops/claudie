package service

// import (
// 	"fmt"
// 	"strings"
// 	"testing"

// 	"github.com/berops/claudie/internal/hash"
// 	"github.com/berops/claudie/proto/pb/spec"
// 	"github.com/google/go-cmp/cmp"
// 	"github.com/stretchr/testify/assert"

// 	"google.golang.org/protobuf/proto"
// )

// func Test_rollingUpdate(t *testing.T) {
// 	rngHash := hash.Create(hash.Length)
// 	np := &spec.NodePool{
// 		Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
// 			ServerType:      "type",
// 			Image:           "image",
// 			StorageDiskSize: 50,
// 			Region:          "local",
// 			Zone:            "earth",
// 			Count:           1,
// 			Provider: &spec.Provider{
// 				SpecName:          "local-provider",
// 				CloudProviderName: "local",
// 				ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
// 				Templates:         &spec.TemplateRepository{Repository: "https://github.com/berops/claudie", Path: "path-1", CommitHash: "hash-1"},
// 			},
// 			PublicKey:  "pk",
// 			PrivateKey: "sk",
// 			Cidr:       "cidr",
// 		}},
// 		Name: fmt.Sprintf("np-%s", rngHash),
// 		Nodes: []*spec.Node{
// 			{Name: "node-0", Private: "private", Public: "public", NodeType: spec.NodeType_apiEndpoint, Username: "root"},
// 		},
// 		IsControl: true,
// 	}
// 	np2 := &spec.NodePool{
// 		Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
// 			ServerType:      "type",
// 			Image:           "image",
// 			StorageDiskSize: 50,
// 			Region:          "local",
// 			Zone:            "earth",
// 			Count:           3,
// 			Provider: &spec.Provider{
// 				SpecName:          "local-provider",
// 				CloudProviderName: "local",
// 				ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
// 				Templates:         &spec.TemplateRepository{Repository: "https://github.com/berops/claudie", Path: "path-1", CommitHash: "hash-1"},
// 			},
// 			PublicKey:  "pk",
// 			PrivateKey: "sk",
// 			Cidr:       "cidr",
// 		}},
// 		Name: fmt.Sprintf("np2-%s", rngHash),
// 		Nodes: []*spec.Node{
// 			{Name: "node-0", Private: "private-1", Public: "public-1", NodeType: spec.NodeType_worker, Username: "root"},
// 			{Name: "node-1", Private: "private-2", Public: "public-2", NodeType: spec.NodeType_worker, Username: "root"},
// 			{Name: "node-2", Private: "private-3", Public: "public-3", NodeType: spec.NodeType_worker, Username: "root"},
// 		},
// 		IsControl: false,
// 	}
// 	np3 := &spec.NodePool{
// 		Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
// 			ServerType:      "type",
// 			Image:           "image",
// 			StorageDiskSize: 50,
// 			Region:          "local",
// 			Zone:            "earth",
// 			Count:           3,
// 			Provider: &spec.Provider{
// 				SpecName:          "local-provider",
// 				CloudProviderName: "local",
// 				ProviderType:      &spec.Provider_Hetzner{Hetzner: &spec.HetznerProvider{Token: "token"}},
// 				Templates:         &spec.TemplateRepository{Repository: "https://github.com/berops/claudie", Path: "path-1", CommitHash: "hash-1"},
// 			},
// 			PublicKey:  "pk",
// 			PrivateKey: "sk",
// 			Cidr:       "cidr",
// 		}},
// 		Name: fmt.Sprintf("np3-%s", rngHash),
// 		Nodes: []*spec.Node{
// 			{Name: "node-0", Private: "private-1", Public: "public-1", NodeType: spec.NodeType_worker, Username: "root"},
// 			{Name: "node-0", Private: "private-1", Public: "public-1", NodeType: spec.NodeType_worker, Username: "root"},
// 			{Name: "node-0", Private: "private-1", Public: "public-1", NodeType: spec.NodeType_worker, Username: "root"},
// 		},
// 		IsControl: true,
// 	}

// 	current := &spec.Clusters{K8S: &spec.K8Scluster{
// 		ClusterInfo: &spec.ClusterInfo{
// 			Name:      "cluster",
// 			Hash:      "hash",
// 			NodePools: []*spec.NodePool{np},
// 		},
// 	}}

// 	type args struct {
// 		current *spec.Clusters
// 		desired *spec.Clusters
// 	}
// 	tests := []struct {
// 		name     string
// 		args     args
// 		want     []*spec.TaskEvent
// 		wantErr  assert.ErrorAssertionFunc
// 		validate func(t *testing.T, args args, got *spec.K8Scluster)
// 	}{
// 		{
// 			name: "ok-no-lbs-rolling-update-api-server",
// 			args: args{
// 				current: current,
// 				desired: func() *spec.Clusters {
// 					c := proto.Clone(current).(*spec.Clusters)
// 					c.K8S.ClusterInfo.NodePools[0].GetDynamicNodePool().Provider.Templates.CommitHash = "hash-6"
// 					return c
// 				}(),
// 			},
// 			want: []*spec.TaskEvent{
// 				{Event: spec.Event_UPDATE, Description: fmt.Sprintf("rolling update: replacing np-%s with np-", rngHash)},
// 				{Event: spec.Event_UPDATE, Description: "rolling update: moving endpoint from old control plane node to a new control plane node"},
// 				{Event: spec.Event_DELETE, Description: fmt.Sprintf("rolling update: deleting nodes from replaced nodepool np-%s", rngHash)},
// 				{Event: spec.Event_UPDATE, Description: fmt.Sprintf("rolling update: deleting infrastructure of deleted nodes from nodepool np-%s", rngHash)},
// 			},
// 			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
// 			validate: func(t *testing.T, args args, got *spec.K8Scluster) {
// 				assert.NotEqual(t, fmt.Sprintf("np-%s", rngHash), got.ClusterInfo.NodePools[0].Name)
// 				assert.Equal(t, spec.NodeType_master, got.ClusterInfo.NodePools[0].Nodes[0].NodeType)
// 				assert.NotEqual(t, "node-0", got.ClusterInfo.NodePools[0].Nodes[0].Name)
// 				assert.Equal(t, "hash-6", got.ClusterInfo.NodePools[0].GetDynamicNodePool().Provider.Templates.CommitHash)
// 				assert.Equal(t, "10.0.0.0/24", got.ClusterInfo.NodePools[0].GetDynamicNodePool().Cidr)
// 				assert.NotEqual(t, current.K8S.ClusterInfo.NodePools[0].GetDynamicNodePool().PrivateKey, got.ClusterInfo.NodePools[0].GetDynamicNodePool().PrivateKey)
// 				assert.NotEqual(t, current.K8S.ClusterInfo.NodePools[0].GetDynamicNodePool().PublicKey, got.ClusterInfo.NodePools[0].GetDynamicNodePool().PublicKey)
// 			},
// 		},
// 		{
// 			name: "ok-no-lbs-rolling-update",
// 			args: args{
// 				current: func() *spec.Clusters {
// 					c := proto.Clone(current).(*spec.Clusters)
// 					c.K8S.ClusterInfo.NodePools = append(c.K8S.ClusterInfo.NodePools, proto.Clone(np2).(*spec.NodePool), proto.Clone(np3).(*spec.NodePool))
// 					return c
// 				}(),
// 				desired: func() *spec.Clusters {
// 					c := proto.Clone(current).(*spec.Clusters)
// 					c.K8S.ClusterInfo.NodePools = append(c.K8S.ClusterInfo.NodePools, proto.Clone(np2).(*spec.NodePool), proto.Clone(np3).(*spec.NodePool))
// 					c.K8S.ClusterInfo.NodePools[1].GetDynamicNodePool().Provider.Templates.CommitHash = "hash-2"
// 					c.K8S.ClusterInfo.NodePools[2].GetDynamicNodePool().Provider.Templates.CommitHash = "hash-2"
// 					return c
// 				}(),
// 			},
// 			want: []*spec.TaskEvent{
// 				{Event: spec.Event_UPDATE, Description: fmt.Sprintf("rolling update: replacing np2-%s with np2-", rngHash)},
// 				{Event: spec.Event_DELETE, Description: fmt.Sprintf("rolling update: deleting nodes from replaced nodepool np2-%s", rngHash)},
// 				{Event: spec.Event_UPDATE, Description: fmt.Sprintf("rolling update: deleting infrastructure of deleted nodes from nodepool np2-%s", rngHash)},
// 				{Event: spec.Event_UPDATE, Description: fmt.Sprintf("rolling update: replacing np3-%s with np3-", rngHash)},
// 				{Event: spec.Event_DELETE, Description: fmt.Sprintf("rolling update: deleting nodes from replaced nodepool np3-%s", rngHash)},
// 				{Event: spec.Event_UPDATE, Description: fmt.Sprintf("rolling update: deleting infrastructure of deleted nodes from nodepool np3-%s", rngHash)},
// 			},
// 			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool { return assert.Nil(t, err) },
// 			validate: func(t *testing.T, args args, got *spec.K8Scluster) {
// 				assert.Equal(t, fmt.Sprintf("np-%s", rngHash), args.current.K8S.ClusterInfo.NodePools[0].Name)
// 				assert.Equal(t, fmt.Sprintf("np2-%s", rngHash), args.current.K8S.ClusterInfo.NodePools[1].Name)
// 				assert.Equal(t, fmt.Sprintf("np3-%s", rngHash), args.current.K8S.ClusterInfo.NodePools[2].Name)

// 				assert.True(t, strings.HasPrefix(got.ClusterInfo.NodePools[0].Name, "np"))
// 				assert.Equal(t, fmt.Sprintf("np-%s", rngHash), got.ClusterInfo.NodePools[0].Name)
// 				assert.Equal(t, spec.NodeType_apiEndpoint, got.ClusterInfo.NodePools[0].Nodes[0].NodeType)
// 				assert.Equal(t, "node-0", got.ClusterInfo.NodePools[0].Nodes[0].Name)
// 				assert.Equal(t, "hash-1", got.ClusterInfo.NodePools[0].GetDynamicNodePool().Provider.Templates.CommitHash)
// 				assert.Equal(t, "cidr", got.ClusterInfo.NodePools[0].GetDynamicNodePool().Cidr)
// 				assert.Equal(t, current.K8S.ClusterInfo.NodePools[0].GetDynamicNodePool().PrivateKey, got.ClusterInfo.NodePools[0].GetDynamicNodePool().PrivateKey)
// 				assert.Equal(t, current.K8S.ClusterInfo.NodePools[0].GetDynamicNodePool().PublicKey, got.ClusterInfo.NodePools[0].GetDynamicNodePool().PublicKey)

// 				assert.True(t, strings.HasPrefix(got.ClusterInfo.NodePools[1].Name, "np2"))
// 				assert.NotEqual(t, fmt.Sprintf("np2-%s", rngHash), got.ClusterInfo.NodePools[1].Name)
// 				assert.Equal(t, spec.NodeType_worker, got.ClusterInfo.NodePools[1].Nodes[0].NodeType)
// 				assert.Equal(t, spec.NodeType_worker, got.ClusterInfo.NodePools[1].Nodes[1].NodeType)
// 				assert.Equal(t, spec.NodeType_worker, got.ClusterInfo.NodePools[1].Nodes[2].NodeType)
// 				assert.NotEqual(t, "node-0", got.ClusterInfo.NodePools[1].Nodes[0].Name)
// 				assert.NotEqual(t, "node-1", got.ClusterInfo.NodePools[1].Nodes[1].Name)
// 				assert.NotEqual(t, "node-2", got.ClusterInfo.NodePools[1].Nodes[2].Name)
// 				assert.Equal(t, "hash-2", got.ClusterInfo.NodePools[1].GetDynamicNodePool().Provider.Templates.CommitHash)
// 				assert.Equal(t, "10.0.0.0/24", got.ClusterInfo.NodePools[1].GetDynamicNodePool().Cidr)
// 				assert.NotEqual(t, np2.GetDynamicNodePool().PrivateKey, got.ClusterInfo.NodePools[1].GetDynamicNodePool().PrivateKey)
// 				assert.NotEqual(t, np3.GetDynamicNodePool().PublicKey, got.ClusterInfo.NodePools[1].GetDynamicNodePool().PublicKey)

// 				assert.True(t, strings.HasPrefix(got.ClusterInfo.NodePools[2].Name, "np3"))
// 				assert.NotEqual(t, fmt.Sprintf("np3-%s", rngHash), got.ClusterInfo.NodePools[2].Name)
// 				assert.Equal(t, spec.NodeType_master, got.ClusterInfo.NodePools[2].Nodes[0].NodeType)
// 				assert.Equal(t, spec.NodeType_master, got.ClusterInfo.NodePools[2].Nodes[1].NodeType)
// 				assert.Equal(t, spec.NodeType_master, got.ClusterInfo.NodePools[2].Nodes[2].NodeType)
// 				assert.NotEqual(t, "node-0", got.ClusterInfo.NodePools[2].Nodes[0].Name)
// 				assert.NotEqual(t, "node-1", got.ClusterInfo.NodePools[2].Nodes[1].Name)
// 				assert.NotEqual(t, "node-2", got.ClusterInfo.NodePools[2].Nodes[2].Name)
// 				assert.Equal(t, "hash-2", got.ClusterInfo.NodePools[2].GetDynamicNodePool().Provider.Templates.CommitHash)
// 				assert.Equal(t, "10.0.1.0/24", got.ClusterInfo.NodePools[2].GetDynamicNodePool().Cidr)
// 				assert.NotEqual(t, np2.GetDynamicNodePool().PrivateKey, got.ClusterInfo.NodePools[2].GetDynamicNodePool().PrivateKey)
// 				assert.NotEqual(t, np3.GetDynamicNodePool().PublicKey, got.ClusterInfo.NodePools[2].GetDynamicNodePool().PublicKey)
// 			},
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			t.Parallel()

// 			gotK8s, got, err := rollingUpdate(tt.args.current, tt.args.desired)
// 			tt.wantErr(t, err)

// 			assert.Equal(t, len(tt.want), len(got))
// 			for i := range got {
// 				assert.Equal(t, tt.want[i].Event, got[i].Event)

// 				// since we don't know the hash check for prefix
// 				if !assert.True(t, strings.HasPrefix(got[i].Description, tt.want[i].Description)) {
// 					assert.Equal(t, tt.want[i].Description, got[i].Description)
// 				}
// 			}

// 			tt.validate(t, tt.args, gotK8s.K8S)
// 		})
// 	}
// }

// func Test_transferTemplatesRepo(t *testing.T) {
// 	type args struct {
// 		into []*spec.NodePool
// 		from []*spec.NodePool
// 	}
// 	tests := []struct {
// 		name string
// 		args args
// 		want []*spec.NodePool
// 	}{
// 		{
// 			name: "should-pass",
// 			args: args{
// 				into: []*spec.NodePool{
// 					{
// 						Name: "np-1",
// 						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
// 							Provider: &spec.Provider{Templates: &spec.TemplateRepository{
// 								Repository: "repo-1",
// 								Tag:        strPtr("123"),
// 								Path:       "path",
// 								CommitHash: "hash-1",
// 							}}},
// 						},
// 					},
// 					{
// 						Name: "np-2",
// 						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
// 							Provider: &spec.Provider{Templates: &spec.TemplateRepository{
// 								Repository: "repo-2",
// 								Path:       "path",
// 								CommitHash: "hash-2",
// 							}}},
// 						},
// 					},
// 					{
// 						Name: "np-3",
// 						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
// 							Provider: &spec.Provider{Templates: &spec.TemplateRepository{
// 								Repository: "repo-3",
// 								Path:       "path",
// 								CommitHash: "hash-3",
// 							}}},
// 						},
// 					},
// 				},
// 				from: []*spec.NodePool{
// 					{
// 						Name: "np-1",
// 						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
// 							Provider: &spec.Provider{Templates: &spec.TemplateRepository{
// 								Repository: "repo-5",
// 								Tag:        strPtr("421"),
// 								Path:       "path",
// 								CommitHash: "hash-5",
// 							}}},
// 						},
// 					},
// 					{
// 						Name: "np-2",
// 						Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
// 							Provider: &spec.Provider{Templates: &spec.TemplateRepository{
// 								Repository: "repo-4",
// 								Path:       "path",
// 								CommitHash: "hash-4",
// 							}}},
// 						},
// 					},
// 				},
// 			},
// 			want: []*spec.NodePool{
// 				{
// 					Name: "np-1",
// 					Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
// 						Provider: &spec.Provider{Templates: &spec.TemplateRepository{
// 							Repository: "repo-5",
// 							Tag:        strPtr("421"),
// 							Path:       "path",
// 							CommitHash: "hash-5",
// 						}}},
// 					},
// 				},
// 				{
// 					Name: "np-2",
// 					Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
// 						Provider: &spec.Provider{Templates: &spec.TemplateRepository{
// 							Repository: "repo-4",
// 							Path:       "path",
// 							CommitHash: "hash-4",
// 						}}},
// 					},
// 				},
// 				{
// 					Name: "np-3",
// 					Type: &spec.NodePool_DynamicNodePool{DynamicNodePool: &spec.DynamicNodePool{
// 						Provider: &spec.Provider{Templates: &spec.TemplateRepository{
// 							Repository: "repo-3",
// 							Path:       "path",
// 							CommitHash: "hash-3",
// 						}}},
// 					},
// 				},
// 			},
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			t.Parallel()

// 			transferTemplatesRepo(tt.args.into, tt.args.from)
// 			assert.Equal(t, len(tt.want), len(tt.args.into))
// 			for i := range tt.want {
// 				if !assert.True(t, proto.Equal(tt.want[i], tt.args.into[i])) {
// 					if diff := cmp.Diff(tt.want[i], tt.args.into[i]); diff != "" {
// 						t.Fatalf("transferTemplatesRepo() = %v", diff)
// 					}
// 				}
// 			}
// 		})
// 	}
// }
