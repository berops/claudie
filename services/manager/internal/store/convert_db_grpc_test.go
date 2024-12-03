package store_test

import (
	"testing"
	"time"

	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
)

var opts = cmpopts.IgnoreUnexported(
	spec.Config{},
	spec.KubernetesContext{},
	spec.Manifest{},
	spec.ClusterState{},
	spec.Clusters{},
	spec.Events{},
	spec.Workflow{},
	spec.K8Scluster{},
	spec.LoadBalancers{},
	spec.LBcluster{},
	spec.ClusterInfo{},
	spec.NodePool{},
	spec.Node{},
	spec.NodePool_DynamicNodePool{},
	spec.NodePool_StaticNodePool{},
	spec.DynamicNodePool{},
	spec.StaticNodePool{},
	spec.Provider{},
	spec.Provider_Hetzner{},
	spec.HetznerProvider{},
	spec.TemplateRepository{},
	spec.Retry{},
)

func TestConvertToGRPCAndBack(t *testing.T) {
	t.Parallel()

	want := &store.Config{
		Version: 256,
		Name:    "Test-03",
		K8SCtx: store.KubernetesContext{
			Name:      "test-03",
			Namespace: "test-04",
		},
		Manifest: store.Manifest{
			Raw:                 "random-manifest",
			Checksum:            hash.Digest("random-manifest"),
			LastAppliedChecksum: nil,
			State:               manifest.Pending.String(),
		},
		Clusters: map[string]*store.ClusterState{
			"test-03": {
				Current: store.Clusters{},
				Desired: store.Clusters{
					K8s: []byte{10, 223, 1, 10, 24, 68, 101, 115, 105, 114, 101, 100, 45, 75, 56, 115, 45, 116, 101, 115, 116, 45, 99, 108, 117, 115, 116, 101, 114, 18, 4, 97, 98, 99, 100, 42, 188, 1, 26, 13, 116, 101, 115, 116, 45, 110, 111, 100, 101, 112, 111, 111, 108, 34, 46, 10, 12, 116, 101, 115, 116, 45, 110, 111, 100, 101, 45, 48, 49, 18, 11, 49, 57, 50, 46, 49, 54, 56, 46, 48, 46, 49, 26, 9, 49, 50, 55, 46, 48, 46, 48, 46, 49, 32, 1, 42, 4, 114, 111, 111, 116, 40, 1, 10, 121, 10, 11, 112, 101, 114, 102, 111, 114, 109, 97, 110, 99, 101, 18, 6, 108, 97, 116, 101, 115, 116, 24, 50, 34, 5, 108, 111, 99, 97, 108, 42, 5, 108, 111, 99, 97, 108, 48, 3, 58, 48, 10, 7, 104, 101, 116, 122, 110, 101, 114, 18, 10, 104, 101, 116, 122, 110, 101, 114, 45, 48, 49, 106, 17, 10, 6, 47, 114, 111, 111, 116, 47, 26, 7, 104, 101, 116, 122, 110, 101, 114, 34, 6, 10, 4, 116, 101, 115, 116, 90, 7, 100, 101, 102, 97, 117, 108, 116, 98, 7, 100, 101, 102, 97, 117, 108, 116, 114, 12, 49, 50, 55, 46, 48, 46, 48, 46, 49, 47, 50, 52, 18, 12, 49, 50, 55, 46, 48, 46, 48, 46, 49, 47, 50, 52, 26, 15, 116, 101, 115, 116, 45, 107, 117, 98, 101, 99, 111, 110, 102, 105, 103, 34, 15, 116, 101, 115, 116, 45, 107, 117, 98, 101, 114, 110, 101, 116, 101, 115},

					LoadBalancers: []byte{10, 221, 1, 10, 218, 1, 10, 23, 68, 101, 115, 105, 114, 101, 100, 45, 108, 98, 45, 116, 101, 115, 116, 45, 99, 108, 117, 115, 116, 101, 114, 18, 4, 97, 98, 99, 100, 42, 184, 1, 26, 13, 116, 101, 115, 116, 45, 110, 111, 100, 101, 112, 111, 111, 108, 34, 44, 10, 12, 116, 101, 115, 116, 45, 110, 111, 100, 101, 45, 48, 49, 18, 11, 49, 57, 50, 46, 49, 54, 56, 46, 48, 46, 49, 26, 9, 49, 50, 55, 46, 48, 46, 48, 46, 49, 42, 4, 114, 111, 111, 116, 10, 121, 10, 11, 112, 101, 114, 102, 111, 114, 109, 97, 110, 99, 101, 18, 6, 108, 97, 116, 101, 115, 116, 24, 50, 34, 5, 108, 111, 99, 97, 108, 42, 5, 108, 111, 99, 97, 108, 48, 3, 58, 48, 10, 7, 104, 101, 116, 122, 110, 101, 114, 18, 10, 104, 101, 116, 122, 110, 101, 114, 45, 48, 49, 106, 17, 10, 6, 47, 114, 111, 111, 116, 47, 26, 7, 104, 101, 116, 122, 110, 101, 114, 34, 6, 10, 4, 116, 101, 115, 116, 90, 7, 100, 101, 102, 97, 117, 108, 116, 98, 7, 100, 101, 102, 97, 117, 108, 116, 114, 12, 49, 50, 55, 46, 48, 46, 48, 46, 49, 47, 50, 52},
				},
				Events: store.Events{
					TaskEvents: []store.TaskEvent{
						{
							Id:          uuid.New().String(),
							Timestamp:   time.Now().UTC().Format(time.RFC3339),
							Event:       spec.Event_CREATE.String(),
							Task:        []uint8{},
							Description: "Testing",
							OnError:     []uint8{18, 88, 10, 36, 102, 57, 98, 49, 53, 55, 97, 101, 45, 100, 97, 100, 54, 45, 52, 50, 52, 101, 45, 97, 97, 102, 56, 45, 57, 52, 99, 102, 97, 57, 57, 102, 56, 99, 56, 51, 18, 12, 8, 201, 198, 212, 183, 6, 16, 216, 250, 232, 177, 2, 24, 2, 34, 19, 26, 17, 26, 15, 10, 4, 116, 101, 115, 116, 18, 7, 10, 5, 110, 111, 100, 101, 49, 42, 7, 116, 101, 115, 116, 105, 110, 103, 50, 2, 8, 1},
						},
					},
					TTL:        500,
					Autoscaled: true,
				},
				State: store.Workflow{
					Status:      spec.Workflow_DONE.String(),
					Stage:       spec.Workflow_NONE.String(),
					Description: "test",
					Timestamp:   "",
				},
			},
		},
	}

	grpcrepr, err := store.ConvertToGRPC(want)
	if err != nil {
		t.Errorf("failed to convert from database representation to GRPC: %v", err)
	}

	got, err := store.ConvertFromGRPC(grpcrepr)
	if err != nil {
		t.Errorf("failed to convert from GRPC representation to database: %v", err)
	}

	// ignore the Timestamp
	got.Clusters["test-03"].State.Timestamp = ""

	if diff := cmp.Diff(want, got, opts); diff != "" {
		t.Errorf("Conversion DB->GRPC->DB failed\ndiff %v", diff)
	}
}

func TestConvertToDBAndBack(t *testing.T) {
	t.Parallel()

	want := &spec.Config{
		Version: 256,
		Name:    "Test-03",
		K8SCtx: &spec.KubernetesContext{
			Name:      "test-03",
			Namespace: "test-04",
		},
		Manifest: &spec.Manifest{
			Raw:      "random-manifest",
			Checksum: hash.Digest("random-manifest"),
			State:    spec.Manifest_Pending,
		},
		Clusters: map[string]*spec.ClusterState{
			"test-03": {
				Desired: &spec.Clusters{
					K8S: &spec.K8Scluster{
						ClusterInfo: &spec.ClusterInfo{
							Name: "Desired-K8s-test-cluster",
							Hash: "abcd",
							NodePools: []*spec.NodePool{
								{
									Type: &spec.NodePool_DynamicNodePool{
										DynamicNodePool: &spec.DynamicNodePool{
											ServerType:      "performance",
											Image:           "latest",
											StorageDiskSize: 50,
											Region:          "local",
											Zone:            "local",
											Count:           3,
											Provider: &spec.Provider{
												SpecName:          "hetzner",
												CloudProviderName: "hetzner-01",
												ProviderType: &spec.Provider_Hetzner{
													Hetzner: &spec.HetznerProvider{
														Token: "test",
													},
												},
												Templates: &spec.TemplateRepository{
													Repository: "/root/",
													Path:       "hetzner",
												},
											},
											PublicKey:  "default",
											PrivateKey: "default",
											Cidr:       "127.0.0.1/24",
										},
									},
									Name: "test-nodepool",
									Nodes: []*spec.Node{
										{
											Name:     "test-node-01",
											Private:  "192.168.0.1",
											Public:   "127.0.0.1",
											NodeType: spec.NodeType_master,
											Username: "root",
										},
									},
									IsControl: true,
								},
							},
						},
						Network:    "127.0.0.1/24",
						Kubeconfig: "test-kubeconfig",
						Kubernetes: "test-kubernetes",
					},
					LoadBalancers: &spec.LoadBalancers{
						Clusters: []*spec.LBcluster{
							{
								ClusterInfo: &spec.ClusterInfo{
									Name: "Desired-lb-test-cluster",
									Hash: "abcd",
									NodePools: []*spec.NodePool{
										{
											Type: &spec.NodePool_DynamicNodePool{
												DynamicNodePool: &spec.DynamicNodePool{
													ServerType:      "performance",
													Image:           "latest",
													StorageDiskSize: 50,
													Region:          "local",
													Zone:            "local",
													Count:           3,
													Provider: &spec.Provider{
														SpecName:          "hetzner",
														CloudProviderName: "hetzner-01",
														ProviderType: &spec.Provider_Hetzner{
															Hetzner: &spec.HetznerProvider{
																Token: "test",
															},
														},
														Templates: &spec.TemplateRepository{
															Repository: "/root/",
															Path:       "hetzner",
														},
													},
													PublicKey:  "default",
													PrivateKey: "default",
													Cidr:       "127.0.0.1/24",
												},
											},
											Name: "test-nodepool",
											Nodes: []*spec.Node{
												{
													Name:     "test-node-01",
													Private:  "192.168.0.1",
													Public:   "127.0.0.1",
													NodeType: spec.NodeType_worker,
													Username: "root",
												},
											},
											IsControl: false,
										},
									},
								},
								Roles:       nil,
								Dns:         nil,
								TargetedK8S: "",
							},
						},
					},
				},
				Events: &spec.Events{
					Events: nil,
					Ttl:    500,
				},
				State: &spec.Workflow{
					Stage:       spec.Workflow_NONE,
					Status:      spec.Workflow_DONE,
					Description: "test",
				},
			},
		},
	}

	dbrepr, err := store.ConvertFromGRPC(want)
	if err != nil {
		t.Errorf("failed to convert from GRPC to Database representation: %v", err)
	}

	got, err := store.ConvertToGRPC(dbrepr)
	if err != nil {
		t.Errorf("failed to convert from Database to GRPC representation: %v", err)
	}

	if diff := cmp.Diff(want, got, opts); diff != "" {
		t.Errorf("Conversion GRPC->DB->GRPC failed\ndiff %v", diff)
	}
}

func TestConvertFromGRPC(t *testing.T) {
	type args struct {
		cfg *spec.Config
	}
	tests := []struct {
		name    string
		args    args
		want    *store.Config
		wantErr bool
	}{
		{
			name: "check-convert-without-clusters",
			args: args{
				cfg: &spec.Config{
					Version: 256,
					Name:    "Test-01",
					K8SCtx: &spec.KubernetesContext{
						Name:      "test-01",
						Namespace: "test-02",
					},
					Manifest: &spec.Manifest{
						Raw:      "random-manifest",
						Checksum: hash.Digest("random-manifest"),
						State:    spec.Manifest_Pending,
					},
					Clusters: nil,
				},
			},
			want: &store.Config{
				Version: 256,
				Name:    "Test-01",
				K8SCtx: store.KubernetesContext{
					Name:      "test-01",
					Namespace: "test-02",
				},
				Manifest: store.Manifest{
					Raw:                 "random-manifest",
					Checksum:            hash.Digest("random-manifest"),
					LastAppliedChecksum: nil,
					State:               manifest.Pending.String(),
				},
				Clusters: nil,
			},
			wantErr: false,
		},
		{
			name: "check-convert-without-current-state",
			args: args{
				cfg: &spec.Config{
					Version: 256,
					Name:    "Test-03",
					K8SCtx: &spec.KubernetesContext{
						Name:      "test-03",
						Namespace: "test-04",
					},
					Manifest: &spec.Manifest{
						Raw:      "random-manifest",
						Checksum: hash.Digest("random-manifest"),
						State:    spec.Manifest_Pending,
					},
					Clusters: map[string]*spec.ClusterState{
						"test-03": {
							Desired: &spec.Clusters{
								K8S: &spec.K8Scluster{
									ClusterInfo: &spec.ClusterInfo{
										Name: "Desired-K8s-test-cluster",
										Hash: "abcd",
										NodePools: []*spec.NodePool{
											{
												Type: &spec.NodePool_DynamicNodePool{
													DynamicNodePool: &spec.DynamicNodePool{
														ServerType:      "performance",
														Image:           "latest",
														StorageDiskSize: 50,
														Region:          "local",
														Zone:            "local",
														Count:           3,
														Provider: &spec.Provider{
															SpecName:          "hetzner",
															CloudProviderName: "hetzner-01",
															ProviderType: &spec.Provider_Hetzner{
																Hetzner: &spec.HetznerProvider{
																	Token: "test",
																},
															},
															Templates: &spec.TemplateRepository{
																Repository: "/root/",
																Path:       "hetzner",
															},
														},
														PublicKey:  "default",
														PrivateKey: "default",
														Cidr:       "127.0.0.1/24",
													},
												},
												Name: "test-nodepool",
												Nodes: []*spec.Node{
													{
														Name:     "test-node-01",
														Private:  "192.168.0.1",
														Public:   "127.0.0.1",
														NodeType: spec.NodeType_master,
														Username: "root",
													},
												},
												IsControl: true,
											},
										},
									},
									Network:    "127.0.0.1/24",
									Kubeconfig: "test-kubeconfig",
									Kubernetes: "test-kubernetes",
								},
								LoadBalancers: &spec.LoadBalancers{
									Clusters: []*spec.LBcluster{
										{
											ClusterInfo: &spec.ClusterInfo{
												Name: "Desired-lb-test-cluster",
												Hash: "abcd",
												NodePools: []*spec.NodePool{
													{
														Type: &spec.NodePool_DynamicNodePool{
															DynamicNodePool: &spec.DynamicNodePool{
																ServerType:      "performance",
																Image:           "latest",
																StorageDiskSize: 50,
																Region:          "local",
																Zone:            "local",
																Count:           3,
																Provider: &spec.Provider{
																	SpecName:          "hetzner",
																	CloudProviderName: "hetzner-01",
																	ProviderType: &spec.Provider_Hetzner{
																		Hetzner: &spec.HetznerProvider{
																			Token: "test",
																		},
																	},
																	Templates: &spec.TemplateRepository{
																		Repository: "/root/",
																		Path:       "hetzner",
																	},
																},
																PublicKey:  "default",
																PrivateKey: "default",
																Cidr:       "127.0.0.1/24",
															},
														},
														Name: "test-nodepool",
														Nodes: []*spec.Node{
															{
																Name:     "test-node-01",
																Private:  "192.168.0.1",
																Public:   "127.0.0.1",
																NodeType: spec.NodeType_worker,
																Username: "root",
															},
														},
														IsControl: false,
													},
												},
											},
											Roles:       nil,
											Dns:         nil,
											TargetedK8S: "",
										},
									},
								},
							},
							Events: &spec.Events{
								Events: nil,
								Ttl:    500,
							},
							State: &spec.Workflow{
								Stage:       spec.Workflow_NONE,
								Status:      spec.Workflow_DONE,
								Description: "test",
							},
						},
					},
				},
			},
			want: &store.Config{
				Version: 256,
				Name:    "Test-03",
				K8SCtx: store.KubernetesContext{
					Name:      "test-03",
					Namespace: "test-04",
				},
				Manifest: store.Manifest{
					Raw:                 "random-manifest",
					Checksum:            hash.Digest("random-manifest"),
					LastAppliedChecksum: nil,
					State:               manifest.Pending.String(),
				},
				Clusters: map[string]*store.ClusterState{
					"test-03": {
						Current: store.Clusters{},
						Desired: store.Clusters{
							K8s: []byte{10, 223, 1, 10, 24, 68, 101, 115, 105, 114, 101, 100, 45, 75, 56, 115, 45, 116, 101, 115, 116, 45, 99, 108, 117, 115, 116, 101, 114, 18, 4, 97, 98, 99, 100, 42, 188, 1, 26, 13, 116, 101, 115, 116, 45, 110, 111, 100, 101, 112, 111, 111, 108, 34, 46, 10, 12, 116, 101, 115, 116, 45, 110, 111, 100, 101, 45, 48, 49, 18, 11, 49, 57, 50, 46, 49, 54, 56, 46, 48, 46, 49, 26, 9, 49, 50, 55, 46, 48, 46, 48, 46, 49, 32, 1, 42, 4, 114, 111, 111, 116, 40, 1, 10, 121, 10, 11, 112, 101, 114, 102, 111, 114, 109, 97, 110, 99, 101, 18, 6, 108, 97, 116, 101, 115, 116, 24, 50, 34, 5, 108, 111, 99, 97, 108, 42, 5, 108, 111, 99, 97, 108, 48, 3, 58, 48, 10, 7, 104, 101, 116, 122, 110, 101, 114, 18, 10, 104, 101, 116, 122, 110, 101, 114, 45, 48, 49, 106, 17, 10, 6, 47, 114, 111, 111, 116, 47, 26, 7, 104, 101, 116, 122, 110, 101, 114, 34, 6, 10, 4, 116, 101, 115, 116, 90, 7, 100, 101, 102, 97, 117, 108, 116, 98, 7, 100, 101, 102, 97, 117, 108, 116, 114, 12, 49, 50, 55, 46, 48, 46, 48, 46, 49, 47, 50, 52, 18, 12, 49, 50, 55, 46, 48, 46, 48, 46, 49, 47, 50, 52, 26, 15, 116, 101, 115, 116, 45, 107, 117, 98, 101, 99, 111, 110, 102, 105, 103, 34, 15, 116, 101, 115, 116, 45, 107, 117, 98, 101, 114, 110, 101, 116, 101, 115},

							LoadBalancers: []byte{10, 221, 1, 10, 218, 1, 10, 23, 68, 101, 115, 105, 114, 101, 100, 45, 108, 98, 45, 116, 101, 115, 116, 45, 99, 108, 117, 115, 116, 101, 114, 18, 4, 97, 98, 99, 100, 42, 184, 1, 26, 13, 116, 101, 115, 116, 45, 110, 111, 100, 101, 112, 111, 111, 108, 34, 44, 10, 12, 116, 101, 115, 116, 45, 110, 111, 100, 101, 45, 48, 49, 18, 11, 49, 57, 50, 46, 49, 54, 56, 46, 48, 46, 49, 26, 9, 49, 50, 55, 46, 48, 46, 48, 46, 49, 42, 4, 114, 111, 111, 116, 10, 121, 10, 11, 112, 101, 114, 102, 111, 114, 109, 97, 110, 99, 101, 18, 6, 108, 97, 116, 101, 115, 116, 24, 50, 34, 5, 108, 111, 99, 97, 108, 42, 5, 108, 111, 99, 97, 108, 48, 3, 58, 48, 10, 7, 104, 101, 116, 122, 110, 101, 114, 18, 10, 104, 101, 116, 122, 110, 101, 114, 45, 48, 49, 106, 17, 10, 6, 47, 114, 111, 111, 116, 47, 26, 7, 104, 101, 116, 122, 110, 101, 114, 34, 6, 10, 4, 116, 101, 115, 116, 90, 7, 100, 101, 102, 97, 117, 108, 116, 98, 7, 100, 101, 102, 97, 117, 108, 116, 114, 12, 49, 50, 55, 46, 48, 46, 48, 46, 49, 47, 50, 52},
						},
						Events: store.Events{
							TaskEvents: nil,
							TTL:        500,
						},
						State: store.Workflow{
							Status:      spec.Workflow_DONE.String(),
							Stage:       spec.Workflow_NONE.String(),
							Description: "test",
							Timestamp:   "",
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := store.ConvertFromGRPC(tt.args.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalToMongoDBRepr() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// ignore timestamp.
			for _, state := range got.Clusters {
				state.State.Timestamp = ""
			}

			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("MarshalToMongoDBRepr() got = %v, want %v\ndiff %v", got, tt.want, diff)
			}
		})
	}
}

func TestConvertToGRPC(t *testing.T) {
	type args struct {
		cfg *store.Config
	}
	tests := []struct {
		name    string
		args    args
		want    *spec.Config
		wantErr bool
	}{
		{
			name: "check-convert-without-clusters",
			want: &spec.Config{
				Version: 256,
				Name:    "Test-01",
				K8SCtx: &spec.KubernetesContext{
					Name:      "test-01",
					Namespace: "test-02",
				},
				Manifest: &spec.Manifest{
					Raw:      "random-manifest",
					Checksum: hash.Digest("random-manifest"),
					State:    spec.Manifest_Pending,
				},
				Clusters: nil,
			},
			args: args{
				cfg: &store.Config{
					Version: 256,
					Name:    "Test-01",
					K8SCtx: store.KubernetesContext{
						Name:      "test-01",
						Namespace: "test-02",
					},
					Manifest: store.Manifest{
						Raw:                 "random-manifest",
						Checksum:            hash.Digest("random-manifest"),
						LastAppliedChecksum: nil,
						State:               manifest.Pending.String(),
					},
					Clusters: nil,
				},
			},
			wantErr: false,
		},
		{
			name: "check-convert-without-current-state",
			want: &spec.Config{
				Version: 256,
				Name:    "Test-03",
				K8SCtx: &spec.KubernetesContext{
					Name:      "test-03",
					Namespace: "test-04",
				},
				Manifest: &spec.Manifest{
					Raw:      "random-manifest",
					Checksum: hash.Digest("random-manifest"),
					State:    spec.Manifest_Pending,
				},
				Clusters: map[string]*spec.ClusterState{
					"test-03": {
						Desired: &spec.Clusters{
							K8S: &spec.K8Scluster{
								ClusterInfo: &spec.ClusterInfo{
									Name: "Desired-K8s-test-cluster",
									Hash: "abcd",
									NodePools: []*spec.NodePool{
										{
											Type: &spec.NodePool_DynamicNodePool{
												DynamicNodePool: &spec.DynamicNodePool{
													ServerType:      "performance",
													Image:           "latest",
													StorageDiskSize: 50,
													Region:          "local",
													Zone:            "local",
													Count:           3,
													Provider: &spec.Provider{
														SpecName:          "hetzner",
														CloudProviderName: "hetzner-01",
														ProviderType: &spec.Provider_Hetzner{
															Hetzner: &spec.HetznerProvider{
																Token: "test",
															},
														},
														Templates: &spec.TemplateRepository{
															Repository: "/root/",
															Path:       "hetzner",
														},
													},
													PublicKey:  "default",
													PrivateKey: "default",
													Cidr:       "127.0.0.1/24",
												},
											},
											Name: "test-nodepool",
											Nodes: []*spec.Node{
												{
													Name:     "test-node-01",
													Private:  "192.168.0.1",
													Public:   "127.0.0.1",
													NodeType: spec.NodeType_master,
													Username: "root",
												},
											},
											IsControl: true,
										},
									},
								},
								Network:    "127.0.0.1/24",
								Kubeconfig: "test-kubeconfig",
								Kubernetes: "test-kubernetes",
							},
							LoadBalancers: &spec.LoadBalancers{
								Clusters: []*spec.LBcluster{
									{
										ClusterInfo: &spec.ClusterInfo{
											Name: "Desired-lb-test-cluster",
											Hash: "abcd",
											NodePools: []*spec.NodePool{
												{
													Type: &spec.NodePool_DynamicNodePool{
														DynamicNodePool: &spec.DynamicNodePool{
															ServerType:      "performance",
															Image:           "latest",
															StorageDiskSize: 50,
															Region:          "local",
															Zone:            "local",
															Count:           3,
															Provider: &spec.Provider{
																SpecName:          "hetzner",
																CloudProviderName: "hetzner-01",
																ProviderType: &spec.Provider_Hetzner{
																	Hetzner: &spec.HetznerProvider{
																		Token: "test",
																	},
																},
																Templates: &spec.TemplateRepository{
																	Repository: "/root/",
																	Path:       "hetzner",
																},
															},
															PublicKey:  "default",
															PrivateKey: "default",
															Cidr:       "127.0.0.1/24",
														},
													},
													Name: "test-nodepool",
													Nodes: []*spec.Node{
														{
															Name:     "test-node-01",
															Private:  "192.168.0.1",
															Public:   "127.0.0.1",
															NodeType: spec.NodeType_worker,
															Username: "root",
														},
													},
													IsControl: false,
												},
											},
										},
										Roles:       nil,
										Dns:         nil,
										TargetedK8S: "",
									},
								},
							},
						},
						Events: &spec.Events{
							Events: nil,
							Ttl:    500,
						},
						State: &spec.Workflow{
							Stage:       spec.Workflow_NONE,
							Status:      spec.Workflow_DONE,
							Description: "test",
						},
					},
				},
			},
			args: args{
				cfg: &store.Config{
					Version: 256,
					Name:    "Test-03",
					K8SCtx: store.KubernetesContext{
						Name:      "test-03",
						Namespace: "test-04",
					},
					Manifest: store.Manifest{
						Raw:                 "random-manifest",
						Checksum:            hash.Digest("random-manifest"),
						LastAppliedChecksum: nil,
						State:               manifest.Pending.String(),
					},
					Clusters: map[string]*store.ClusterState{
						"test-03": {
							Current: store.Clusters{},
							Desired: store.Clusters{
								K8s: []byte{10, 223, 1, 10, 24, 68, 101, 115, 105, 114, 101, 100, 45, 75, 56, 115, 45, 116, 101, 115, 116, 45, 99, 108, 117, 115, 116, 101, 114, 18, 4, 97, 98, 99, 100, 42, 188, 1, 26, 13, 116, 101, 115, 116, 45, 110, 111, 100, 101, 112, 111, 111, 108, 34, 46, 10, 12, 116, 101, 115, 116, 45, 110, 111, 100, 101, 45, 48, 49, 18, 11, 49, 57, 50, 46, 49, 54, 56, 46, 48, 46, 49, 26, 9, 49, 50, 55, 46, 48, 46, 48, 46, 49, 32, 1, 42, 4, 114, 111, 111, 116, 40, 1, 10, 121, 10, 11, 112, 101, 114, 102, 111, 114, 109, 97, 110, 99, 101, 18, 6, 108, 97, 116, 101, 115, 116, 24, 50, 34, 5, 108, 111, 99, 97, 108, 42, 5, 108, 111, 99, 97, 108, 48, 3, 58, 48, 10, 7, 104, 101, 116, 122, 110, 101, 114, 18, 10, 104, 101, 116, 122, 110, 101, 114, 45, 48, 49, 106, 17, 10, 6, 47, 114, 111, 111, 116, 47, 26, 7, 104, 101, 116, 122, 110, 101, 114, 34, 6, 10, 4, 116, 101, 115, 116, 90, 7, 100, 101, 102, 97, 117, 108, 116, 98, 7, 100, 101, 102, 97, 117, 108, 116, 114, 12, 49, 50, 55, 46, 48, 46, 48, 46, 49, 47, 50, 52, 18, 12, 49, 50, 55, 46, 48, 46, 48, 46, 49, 47, 50, 52, 26, 15, 116, 101, 115, 116, 45, 107, 117, 98, 101, 99, 111, 110, 102, 105, 103, 34, 15, 116, 101, 115, 116, 45, 107, 117, 98, 101, 114, 110, 101, 116, 101, 115},

								LoadBalancers: []byte{10, 221, 1, 10, 218, 1, 10, 23, 68, 101, 115, 105, 114, 101, 100, 45, 108, 98, 45, 116, 101, 115, 116, 45, 99, 108, 117, 115, 116, 101, 114, 18, 4, 97, 98, 99, 100, 42, 184, 1, 26, 13, 116, 101, 115, 116, 45, 110, 111, 100, 101, 112, 111, 111, 108, 34, 44, 10, 12, 116, 101, 115, 116, 45, 110, 111, 100, 101, 45, 48, 49, 18, 11, 49, 57, 50, 46, 49, 54, 56, 46, 48, 46, 49, 26, 9, 49, 50, 55, 46, 48, 46, 48, 46, 49, 42, 4, 114, 111, 111, 116, 10, 121, 10, 11, 112, 101, 114, 102, 111, 114, 109, 97, 110, 99, 101, 18, 6, 108, 97, 116, 101, 115, 116, 24, 50, 34, 5, 108, 111, 99, 97, 108, 42, 5, 108, 111, 99, 97, 108, 48, 3, 58, 48, 10, 7, 104, 101, 116, 122, 110, 101, 114, 18, 10, 104, 101, 116, 122, 110, 101, 114, 45, 48, 49, 106, 17, 10, 6, 47, 114, 111, 111, 116, 47, 26, 7, 104, 101, 116, 122, 110, 101, 114, 34, 6, 10, 4, 116, 101, 115, 116, 90, 7, 100, 101, 102, 97, 117, 108, 116, 98, 7, 100, 101, 102, 97, 117, 108, 116, 114, 12, 49, 50, 55, 46, 48, 46, 48, 46, 49, 47, 50, 52},
							},
							Events: store.Events{
								TaskEvents: nil,
								TTL:        500,
							},
							State: store.Workflow{
								Status:      spec.Workflow_DONE.String(),
								Stage:       spec.Workflow_NONE.String(),
								Description: "test",
								Timestamp:   "",
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := store.ConvertToGRPC(tt.args.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshallFromMongoDBRepr() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if diff := cmp.Diff(got, tt.want, opts); diff != "" {
				t.Errorf("UnmarshallFromMongoDBRepr() got = %v, want %v\ndiff %v", got, tt.want, diff)
			}
		})
	}
}
