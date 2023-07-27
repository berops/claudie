package utils

import (
	"github.com/berops/claudie/proto/pb"
	"github.com/google/go-cmp/cmp"
	"golang.org/x/exp/slices"
	"testing"
)

func TestMergeLbClusters(t *testing.T) {
	type args struct {
		c []*pb.LBcluster
		n map[string][]*pb.LBcluster
	}
	tests := []struct {
		Name string
		args args
		want []*pb.LBcluster
	}{
		{
			Name: "test-set-0",
			args: args{
				c: []*pb.LBcluster{
					{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-0"}, TargetedK8S: "test-set-0"},
					{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-1"}, TargetedK8S: "test-set-0"},
					{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-0"},
					{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-0"}, TargetedK8S: "test-set-1"},
				},
				n: map[string][]*pb.LBcluster{
					"test-set-0": {
						{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-0"}, TargetedK8S: "test-set-0"},
						{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-0"},
						{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-3"}, TargetedK8S: "test-set-0"},
						{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-4"}, TargetedK8S: "test-set-0"},
					},
					"test-set-1": {
						{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-1"},
					},
				},
			},
			want: []*pb.LBcluster{
				{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-0"}, TargetedK8S: "test-set-0"},
				{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-0"},
				{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-3"}, TargetedK8S: "test-set-0"},
				{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-4"}, TargetedK8S: "test-set-0"},
				{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-1"},
			},
		},
		{
			Name: "test-set-1",
			args: args{
				c: []*pb.LBcluster{
					{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-0"}, TargetedK8S: "test-set-0"},
					{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-1"}, TargetedK8S: "test-set-0"},
					{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-0"},
					{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-0"}, TargetedK8S: "test-set-1"},
				},
				n: map[string][]*pb.LBcluster{
					"test-set-0": {
						{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-0"}, TargetedK8S: "test-set-0"},
						{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-1"}, TargetedK8S: "test-set-0"},
						{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-0"},
					},
					"test-set-1": {
						{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-1"},
					},
				},
			},
			want: []*pb.LBcluster{
				{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-0"}, TargetedK8S: "test-set-0"},
				{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-1"}, TargetedK8S: "test-set-0"},
				{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-0"},
				{ClusterInfo: &pb.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			got := mergeLbClusters(tt.args.c, tt.args.n)

			slices.SortFunc(got, func(l, t *pb.LBcluster) int {
				if l.ClusterInfo.Name < t.ClusterInfo.Name {
					return -1
				} else if l.ClusterInfo.Name > t.ClusterInfo.Name {
					return 1
				}
				return 0
			})

			slices.SortFunc(tt.want, func(l, t *pb.LBcluster) int {
				if l.ClusterInfo.Name < t.ClusterInfo.Name {
					return -1
				} else if l.ClusterInfo.Name > t.ClusterInfo.Name {
					return 1
				}
				return 0
			})

			diff := cmp.Diff(got, tt.want, cmp.Comparer(func(l, t *pb.LBcluster) bool {
				return l.ClusterInfo.Name == t.ClusterInfo.Name
			}))

			if diff != "" {
				t.Errorf("%s failed with diff: %s", tt.Name, diff)
			}
		})
	}
}

func TestMergeK8sClusters(t *testing.T) {
	type args struct {
		c []*pb.K8Scluster
		n map[string]*pb.K8Scluster
	}
	tests := []struct {
		Name string
		args args
		want []*pb.K8Scluster
	}{
		{
			Name: "test-case-0",
			args: args{
				c: []*pb.K8Scluster{
					{ClusterInfo: &pb.ClusterInfo{Name: "test-0"}},
					{ClusterInfo: &pb.ClusterInfo{Name: "test-1"}},
				},
				n: map[string]*pb.K8Scluster{
					"test-0": {ClusterInfo: &pb.ClusterInfo{Name: "test-0"}},
					"test-1": {ClusterInfo: &pb.ClusterInfo{Name: "test-1"}},
					"test-2": {ClusterInfo: &pb.ClusterInfo{Name: "test-2"}},
				},
			},
			want: []*pb.K8Scluster{
				{ClusterInfo: &pb.ClusterInfo{Name: "test-0"}},
				{ClusterInfo: &pb.ClusterInfo{Name: "test-1"}},
				{ClusterInfo: &pb.ClusterInfo{Name: "test-2"}},
			},
		},
		{
			Name: "test-case-1",
			args: args{
				c: []*pb.K8Scluster{
					{ClusterInfo: &pb.ClusterInfo{Name: "test-0"}},
					{ClusterInfo: &pb.ClusterInfo{Name: "test-1"}},
				},
				n: map[string]*pb.K8Scluster{
					"test-0": {ClusterInfo: &pb.ClusterInfo{Name: "test-0"}},
					"test-3": {ClusterInfo: &pb.ClusterInfo{Name: "test-3"}},
				},
			},
			want: []*pb.K8Scluster{
				{ClusterInfo: &pb.ClusterInfo{Name: "test-0"}},
				{ClusterInfo: &pb.ClusterInfo{Name: "test-3"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			got := mergeK8sClusters(tt.args.c, tt.args.n)

			diff := cmp.Diff(got, tt.want, cmp.Comparer(func(l, t *pb.K8Scluster) bool {
				return l.ClusterInfo.Name == t.ClusterInfo.Name
			}))

			if diff != "" {
				t.Errorf("%s failed with diff: %s", tt.Name, diff)
			}
		})
	}
}
