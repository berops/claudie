package cluster_builder

import (
	"reflect"
	"testing"

	"github.com/berops/claudie/services/terraformer/server/domain/utils/templates"
)

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
