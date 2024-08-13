package utils

import (
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/google/go-cmp/cmp"
	"golang.org/x/exp/slices"
	"testing"
)

func TestMergeLbClusters(t *testing.T) {
	type args struct {
		c []*spec.LBcluster
		n map[string][]*spec.LBcluster
	}
	tests := []struct {
		Name string
		args args
		want []*spec.LBcluster
	}{
		{
			Name: "test-set-0",
			args: args{
				c: []*spec.LBcluster{
					{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-0"}, TargetedK8S: "test-set-0"},
					{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-1"}, TargetedK8S: "test-set-0"},
					{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-0"},
					{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-0"}, TargetedK8S: "test-set-1"},
				},
				n: map[string][]*spec.LBcluster{
					"test-set-0": {
						{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-0"}, TargetedK8S: "test-set-0"},
						{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-0"},
						{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-3"}, TargetedK8S: "test-set-0"},
						{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-4"}, TargetedK8S: "test-set-0"},
					},
					"test-set-1": {
						{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-1"},
					},
				},
			},
			want: []*spec.LBcluster{
				{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-0"}, TargetedK8S: "test-set-0"},
				{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-0"},
				{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-3"}, TargetedK8S: "test-set-0"},
				{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-4"}, TargetedK8S: "test-set-0"},
				{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-1"},
			},
		},
		{
			Name: "test-set-1",
			args: args{
				c: []*spec.LBcluster{
					{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-0"}, TargetedK8S: "test-set-0"},
					{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-1"}, TargetedK8S: "test-set-0"},
					{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-0"},
					{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-0"}, TargetedK8S: "test-set-1"},
				},
				n: map[string][]*spec.LBcluster{
					"test-set-0": {
						{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-0"}, TargetedK8S: "test-set-0"},
						{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-1"}, TargetedK8S: "test-set-0"},
						{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-0"},
					},
					"test-set-1": {
						{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-1"},
					},
				},
			},
			want: []*spec.LBcluster{
				{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-0"}, TargetedK8S: "test-set-0"},
				{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-1"}, TargetedK8S: "test-set-0"},
				{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-0"},
				{ClusterInfo: &spec.ClusterInfo{Name: "test-set-lb-2"}, TargetedK8S: "test-set-1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			got := mergeLbClusters(tt.args.c, tt.args.n)

			slices.SortFunc(got, func(l, t *spec.LBcluster) int {
				if l.ClusterInfo.Name < t.ClusterInfo.Name {
					return -1
				} else if l.ClusterInfo.Name > t.ClusterInfo.Name {
					return 1
				}
				return 0
			})

			slices.SortFunc(tt.want, func(l, t *spec.LBcluster) int {
				if l.ClusterInfo.Name < t.ClusterInfo.Name {
					return -1
				} else if l.ClusterInfo.Name > t.ClusterInfo.Name {
					return 1
				}
				return 0
			})

			diff := cmp.Diff(got, tt.want, cmp.Comparer(func(l, t *spec.LBcluster) bool {
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
		c []*spec.K8Scluster
		n map[string]*spec.K8Scluster
	}
	tests := []struct {
		Name string
		args args
		want []*spec.K8Scluster
	}{
		{
			Name: "test-case-0",
			args: args{
				c: []*spec.K8Scluster{
					{ClusterInfo: &spec.ClusterInfo{Name: "test-0"}},
					{ClusterInfo: &spec.ClusterInfo{Name: "test-1"}},
				},
				n: map[string]*spec.K8Scluster{
					"test-0": {ClusterInfo: &spec.ClusterInfo{Name: "test-0"}},
					"test-1": {ClusterInfo: &spec.ClusterInfo{Name: "test-1"}},
					"test-2": {ClusterInfo: &spec.ClusterInfo{Name: "test-2"}},
				},
			},
			want: []*spec.K8Scluster{
				{ClusterInfo: &spec.ClusterInfo{Name: "test-0"}},
				{ClusterInfo: &spec.ClusterInfo{Name: "test-1"}},
				{ClusterInfo: &spec.ClusterInfo{Name: "test-2"}},
			},
		},
		{
			Name: "test-case-1",
			args: args{
				c: []*spec.K8Scluster{
					{ClusterInfo: &spec.ClusterInfo{Name: "test-0"}},
					{ClusterInfo: &spec.ClusterInfo{Name: "test-1"}},
				},
				n: map[string]*spec.K8Scluster{
					"test-0": {ClusterInfo: &spec.ClusterInfo{Name: "test-0"}},
					"test-3": {ClusterInfo: &spec.ClusterInfo{Name: "test-3"}},
				},
			},
			want: []*spec.K8Scluster{
				{ClusterInfo: &spec.ClusterInfo{Name: "test-0"}},
				{ClusterInfo: &spec.ClusterInfo{Name: "test-3"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			got := mergeK8sClusters(tt.args.c, tt.args.n)

			diff := cmp.Diff(got, tt.want, cmp.Comparer(func(l, t *spec.K8Scluster) bool {
				return l.ClusterInfo.Name == t.ClusterInfo.Name
			}))

			if diff != "" {
				t.Errorf("%s failed with diff: %s", tt.Name, diff)
			}
		})
	}
}
