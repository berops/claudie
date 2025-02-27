package clusters

import (
	"errors"
	"math/rand/v2"
	"net/netip"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/berops/claudie/internal/spectesting"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func FuzzPingAll(f *testing.F) {
	testcases := []string{
		"1,2,3",
		"1,2,3,4,5",
		"1,2,3,4,5,6,7,8,9,10",
		"1,2,3,4,5,6,7,8,9,16,17,18",
	}
	for _, tc := range testcases {
		f.Add(tc)
	}
	log := zerolog.Logger{}

	f.Fuzz(func(t *testing.T, s string) {
		ips := strings.Split(s, ",")
		gc := rand.IntN(20)
		u, err := pingAll(log, gc, ips, func(logger zerolog.Logger, count int, dst string) error { return nil })
		if err != nil {
			t.Errorf("pingAll() goroutines = %v, ips = %v, unreachable = %v, err = %v", gc, ips, u, err)
		}
	})
}

func TestPingNodes(t *testing.T) {
	logger := zerolog.New(os.Stdout)

	var (
		wantK8sIps = make(map[string][]string)
		wantLbsIps = make(map[string][]string)
	)

	// re-assign ips so that no two nodes have the same ip.
	network, err := netip.ParsePrefix("172.0.1.0/16")
	assert.Nil(t, err)
	iter := network.Addr()

	k8s := spectesting.GenerateFakeK8SCluster(true)
	k8s.ClusterInfo.NodePools = k8s.ClusterInfo.NodePools[:2]
	for _, np := range k8s.ClusterInfo.NodePools {
		np.Nodes = np.Nodes[:2]
		np.Nodes[0].Public = iter.String()
		iter = iter.Next()

		np.Nodes[1].Public = iter.String()
		iter = iter.Next()
		wantK8sIps[np.Name] = append(wantK8sIps[np.Name], np.Nodes[0].Public)
		wantK8sIps[np.Name] = append(wantK8sIps[np.Name], np.Nodes[1].Public)
	}

	lbs := spectesting.GenerateFakeLBCluster(true, k8s.ClusterInfo)
	lbs.ClusterInfo.NodePools = lbs.ClusterInfo.NodePools[:2]
	for _, np := range lbs.ClusterInfo.NodePools {
		np.Nodes = np.Nodes[:2]
		np.Nodes[0].Public = iter.String()
		iter = iter.Next()

		np.Nodes[1].Public = iter.String()
		iter = iter.Next()
		wantLbsIps[np.Name] = append(wantLbsIps[np.Name], np.Nodes[0].Public)
		wantLbsIps[np.Name] = append(wantLbsIps[np.Name], np.Nodes[1].Public)
	}
	s := &spec.Clusters{
		K8S:           k8s,
		LoadBalancers: &spec.LoadBalancers{Clusters: []*spec.LBcluster{lbs}},
	}

	gotK8s, gotLbs, err := PingNodes(logger, s)
	assert.NotNil(t, err)

	assert.Equal(t, 2, len(gotK8s))
	assert.Equal(t, 1, len(gotLbs))

	assert.Equal(t, 2, len(gotK8s[k8s.ClusterInfo.NodePools[0].Name]))
	assert.Equal(t, 2, len(gotK8s[k8s.ClusterInfo.NodePools[1].Name]))

	assert.Equal(t, 2, len(gotLbs[lbs.ClusterInfo.Id()][lbs.ClusterInfo.NodePools[0].Name]))
	assert.Equal(t, 2, len(gotLbs[lbs.ClusterInfo.Id()][lbs.ClusterInfo.NodePools[1].Name]))

	for np, v := range wantK8sIps {
		for _, ip := range v {
			assert.True(t, slices.Contains(gotK8s[np], ip))
		}
	}

	for np, v := range wantLbsIps {
		for _, ip := range v {
			assert.True(t, slices.Contains(gotLbs[lbs.ClusterInfo.Id()][np], ip))
		}
	}
}

func TestPingAll(t *testing.T) {
	logger := zerolog.Logger{}
	type args struct {
		goroutineCount int
		ips            []string
		f              func(logger zerolog.Logger, count int, dst string) error
	}

	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			args: args{
				goroutineCount: 0,
				ips:            []string{},
				f:              func(logger zerolog.Logger, count int, dst string) error { return nil },
			},
		},
		{
			args: args{
				goroutineCount: 3,
				ips:            []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"},
				f:              func(logger zerolog.Logger, count int, dst string) error { return nil },
			},
			want:    nil,
			wantErr: false,
		},
		{
			args: args{
				goroutineCount: 3,
				ips:            []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"},
				f: func(logger zerolog.Logger, count int, dst string) error {
					switch dst {
					case "1", "2", "3":
						return errors.New("not reachable")
					}
					return nil
				},
			},
			want:    []string{"1", "2", "3"},
			wantErr: true,
		},
		{
			args: args{
				goroutineCount: 3,
				ips:            []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"},
				f: func(logger zerolog.Logger, count int, dst string) error {
					switch dst {
					case "1", "2", "6", "10":
						return errors.New("not reachable")
					}
					return nil
				},
			},
			want:    []string{"1", "2", "6", "10"},
			wantErr: true,
		},
		{
			args: args{
				goroutineCount: 3,
				ips:            []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
				f: func(logger zerolog.Logger, count int, dst string) error {
					switch dst {
					case "1", "2", "6", "10":
						return errors.New("not reachable")
					}
					return nil
				},
			},
			want:    []string{"1", "2", "6", "10"},
			wantErr: true,
		},
		{
			args: args{
				goroutineCount: 3,
				ips:            []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"},
				f: func(logger zerolog.Logger, count int, dst string) error {
					switch dst {
					case "1", "2", "6", "10":
						return errors.New("not reachable")
					}
					return nil
				},
			},
			want:    []string{"1", "2", "6", "10"},
			wantErr: true,
		},
		{
			args: args{
				goroutineCount: 3,
				ips:            []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19"},
				f: func(logger zerolog.Logger, count int, dst string) error {
					switch dst {
					case "1", "2", "6", "10":
						return errors.New("not reachable")
					}
					return nil
				},
			},
			want:    []string{"1", "2", "6", "10"},
			wantErr: true,
		},
		{
			args: args{
				goroutineCount: 3,
				ips:            []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19", "20", "21", "22", "23", "24", "25", "26", "27", "28"},
				f: func(logger zerolog.Logger, count int, dst string) error {
					switch dst {
					case "1", "2", "6", "10":
						return errors.New("not reachable")
					}
					return nil
				},
			},
			want:    []string{"1", "2", "6", "10"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := pingAll(logger, tt.args.goroutineCount, tt.args.ips, tt.args.f)
			if (gotErr != nil) != tt.wantErr {
				t.Fatalf("pingAll() got %v, want %v", gotErr, tt.wantErr)
				return
			}

			for _, got := range got {
				found := false
				for _, want := range tt.want {
					if want == got {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("pingAll() got %v missing in want %v", got, tt.want)
				}
			}
		})
	}
}
