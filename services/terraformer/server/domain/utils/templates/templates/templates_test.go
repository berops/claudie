package templates_test

import (
	"os"
	"testing"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/templates/templates"
)

func TestDownloadForNodepools(t *testing.T) {
	downloadDir := "./test"
	t.Cleanup(func() { os.RemoveAll(downloadDir) })

	type args struct {
		downloadInto string
		nodepools    []*pb.NodePool
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "test-case-0",
			wantErr: false,
			args: args{
				downloadInto: downloadDir,
				nodepools: []*pb.NodePool{
					{
						NodePoolType: &pb.NodePool_DynamicNodePool{
							DynamicNodePool: &pb.DynamicNodePool{
								Templates: &pb.TemplateRepository{
									Repository: "https://github.com/Despire/claudie-config",
									Tag:        "v0.1.0",
									Path:       "/templates",
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "test-case-1",
			wantErr: false,
			args: args{
				downloadInto: downloadDir,
				nodepools: []*pb.NodePool{
					{
						NodePoolType: &pb.NodePool_DynamicNodePool{
							DynamicNodePool: &pb.DynamicNodePool{
								Templates: &pb.TemplateRepository{
									Repository: "https://github.com/berops/claudie-config",
									Tag:        "v0.1.0",
									Path:       "/templates",
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "test-case-2",
			wantErr: true,
			args: args{
				downloadInto: downloadDir,
				nodepools: []*pb.NodePool{
					{
						NodePoolType: &pb.NodePool_DynamicNodePool{
							DynamicNodePool: &pb.DynamicNodePool{
								Templates: &pb.TemplateRepository{
									Repository: "https://github.com/berops/claudie-config",
									Tag:        "v0.0.0",
									Path:       "/templates",
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "test-case-3",
			wantErr: true,
			args: args{
				downloadInto: downloadDir,
				nodepools: []*pb.NodePool{
					{
						NodePoolType: &pb.NodePool_DynamicNodePool{
							DynamicNodePool: &pb.DynamicNodePool{},
						},
					},
				},
			},
		},
		{
			name:    "test-case-4",
			wantErr: true,
			args: args{
				downloadInto: downloadDir,
				nodepools: []*pb.NodePool{
					{
						NodePoolType: &pb.NodePool_DynamicNodePool{
							DynamicNodePool: &pb.DynamicNodePool{
								Templates: &pb.TemplateRepository{
									Repository: "h??ttps:/github.com/berops/claudie-config",
									Tag:        "v0.1.0",
									Path:       "/templates",
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "test-case-5",
			wantErr: false,
			args: args{
				downloadInto: downloadDir,
				nodepools: []*pb.NodePool{
					{
						NodePoolType: &pb.NodePool_DynamicNodePool{
							DynamicNodePool: &pb.DynamicNodePool{
								Templates: &pb.TemplateRepository{
									Repository: "https://github.com/berops/claudie-config",
									Tag:        "v0.1.0",
									Path:       "/templates",
								},
							},
						},
					},
					{
						NodePoolType: &pb.NodePool_DynamicNodePool{
							DynamicNodePool: &pb.DynamicNodePool{
								Templates: &pb.TemplateRepository{
									Repository: "https://github.com/berops/claudie-config",
									Tag:        "v0.1.0",
									Path:       "/templates",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := templates.DownloadForNodepools(tt.args.downloadInto, tt.args.nodepools)
			if (err != nil) != tt.wantErr {
				t.Errorf("templates.DownloadForNodepools() = %v", err)
			}
		})
	}
}
