package cluster_builder

import (
	"reflect"
	"testing"

	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/templates"
)

func Test_readNodeOutput(t *testing.T) {
	type args struct {
		data string
	}
	tests := []struct {
		name    string
		args    args
		want    templates.NodepoolOutput
		wantErr bool
	}{
		{
			name: "test-01",
			args: args{
				data: "{\"test-cluster-compute1\":\"0.0.0.65\",\n\"test-cluster-compute2\":\"0.0.0.512\", \"test-cluster-control1\":\"0.0.0.72\",\n\"test-cluster-control2\":\"0.0.0.65\"}",
			},
			want: templates.NodepoolOutput{
				Nodes: map[string]any{
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
			got, err := readNodeOutput(tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("readNodeOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readNodeOutput() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseNodeOutput(t *testing.T) {
	tests := []struct {
		name     string
		val      any
		wantIP   string
		wantPort string
		wantErr  bool
	}{
		{
			name:     "string IP only",
			val:      "18.194.81.150",
			wantIP:   "18.194.81.150",
			wantPort: "",
		},
		{
			name:     "array with IP and port string",
			val:      []any{"18.194.81.150", "22"},
			wantIP:   "18.194.81.150",
			wantPort: "22",
		},
		{
			name:     "array with IP and port number",
			val:      []any{"18.194.81.150", float64(22522)},
			wantIP:   "18.194.81.150",
			wantPort: "22522",
		},
		{
			name:     "array with IP only",
			val:      []any{"18.194.81.150"},
			wantIP:   "18.194.81.150",
			wantPort: "",
		},
		{
			name:    "empty array",
			val:     []any{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, port, err := parseNodeOutput(tt.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseNodeOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if ip != tt.wantIP {
				t.Errorf("parseNodeOutput() ip = %v, want %v", ip, tt.wantIP)
			}
			if port != tt.wantPort {
				t.Errorf("parseNodeOutput() port = %v, want %v", port, tt.wantPort)
			}
		})
	}
}
