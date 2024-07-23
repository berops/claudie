package cluster_builder

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/templates/templates"
)

// TestGetCIDR tests getCIDR function
func TestGetCIDR(t *testing.T) {
	type testCase struct {
		desc     string
		baseCIDR string
		position int
		existing map[string]struct{}
		out      string
	}

	testDataSucc := []testCase{
		{
			desc:     "Second octet change",
			baseCIDR: "10.0.0.0/24",
			position: 1,
			existing: map[string]struct{}{
				"10.1.0.0/24": {},
			},
			out: "10.0.0.0/24",
		},

		{
			desc:     "Third octet change",
			baseCIDR: "10.0.0.0/24",
			position: 2,
			existing: map[string]struct{}{
				"10.0.0.0/24": {},
			},
			out: "10.0.1.0/24",
		},
	}
	for _, test := range testDataSucc {
		if out, err := getCIDR(test.baseCIDR, test.position, test.existing); out != test.out || err != nil {
			t.Error(test.desc, err, out)
		}
	}
	testDataFail := []testCase{
		{
			desc:     "Max IP error",
			baseCIDR: "10.0.0.0/24",
			position: 2,
			existing: func() map[string]struct{} {
				m := make(map[string]struct{})
				for i := 0; i < 256; i++ {
					m[fmt.Sprintf("10.0.%d.0/24", i)] = struct{}{}
				}
				return m
			}(),
			out: "",
		},
		{
			desc:     "Invalid base CIDR",
			baseCIDR: "300.0.0.0/24",
			position: 2,
			existing: map[string]struct{}{
				"10.0.0.0/24": {},
			},
			out: "10.0.10.0/24",
		},
	}
	for _, test := range testDataFail {
		if _, err := getCIDR(test.baseCIDR, test.position, test.existing); err == nil {
			t.Error(test.desc, "test should have failed, but was successful")
		} else {
			t.Log(err)
		}
	}
}

func Test_calculateCIDR(t *testing.T) {
	type args struct {
		baseCIDR  string
		nodepools []*pb.DynamicNodePool
	}
	tests := []struct {
		name      string
		args      args
		wantErr   bool
		wantCidrs []string
	}{
		{
			name: "test-01",
			args: args{
				baseCIDR: baseSubnetCIDR,
				nodepools: []*pb.DynamicNodePool{
					{Cidr: ""},
					{Cidr: ""},
					{Cidr: ""},
				},
			},
			wantErr: false,
			wantCidrs: []string{
				"10.0.0.0/24",
				"10.0.1.0/24",
				"10.0.2.0/24",
			},
		},
		{
			name: "test-02",
			args: args{
				baseCIDR: baseSubnetCIDR,
				nodepools: []*pb.DynamicNodePool{
					{Cidr: "10.0.0.0/24"},
					{Cidr: "10.0.2.0/24"},
				},
			},
			wantErr: false,
			wantCidrs: []string{
				"10.0.0.0/24",
				"10.0.2.0/24",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := calculateCIDR(tt.args.baseCIDR, tt.args.nodepools); (err != nil) != tt.wantErr {
				t.Errorf("calculateCIDR() error = %v, wantErr %v", err, tt.wantErr)
			}
			for i, cidr := range tt.wantCidrs {
				if tt.args.nodepools[i].Cidr != cidr {
					t.Errorf("calculateCIDR() error = %v want %v", tt.args.nodepools[i].Cidr, cidr)
				}
			}
		})
	}
}

func Test_readIPs(t *testing.T) {
	type args struct {
		data string
	}
	tests := []struct {
		name    string
		args    args
		want    templates.NodepoolIPs
		wantErr bool
	}{
		{
			name: "test-01",
			args: args{
				data: "{\"test-cluster-compute1\":\"0.0.0.65\",\n\"test-cluster-compute2\":\"0.0.0.512\", \"test-cluster-control1\":\"0.0.0.72\",\n\"test-cluster-control2\":\"0.0.0.65\"}",
			},
			want: templates.NodepoolIPs{
				IPs: map[string]any{
					"test-cluster-compute1": "0.0.0.65",
					"test-cluster-compute2": "0.0.0.512",
					"test-cluster-control1": "0.0.0.72",
					"test-cluster-control2": "0.0.0.65",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readIPs(tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("readIPs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readIPs() got = %v, want %v", got, tt.want)
			}
		})
	}
}
