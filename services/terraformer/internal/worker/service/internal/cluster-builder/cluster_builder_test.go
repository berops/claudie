package cluster_builder

import (
	"reflect"
	"testing"

	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/templates"
)

func Test_parseNodeOutput(t *testing.T) {
	tests := []struct {
		name        string
		val         any
		wantIP      string
		wantSSHPort int32
		wantWGPort  int32
		wantErr     bool
	}{
		{name: "legacy string IP", val: "1.2.3.4", wantIP: "1.2.3.4"},
		// JSON numbers decode to float64, so mimic that for the array cases.
		{name: "ip + ssh port (numbers)", val: []any{"1.2.3.4", float64(22522)}, wantIP: "1.2.3.4", wantSSHPort: 22522},
		{name: "ip + ssh + wg ports (numbers)", val: []any{"1.2.3.4", float64(22222), float64(41234)}, wantIP: "1.2.3.4", wantSSHPort: 22222, wantWGPort: 41234},
		// CloudRift template emits ports via tostring(), so they arrive as strings.
		{name: "ip + ssh + wg ports (strings)", val: []any{"1.2.3.4", "22222", "41234"}, wantIP: "1.2.3.4", wantSSHPort: 22222, wantWGPort: 41234},
		{name: "ip only array", val: []any{"1.2.3.4"}, wantIP: "1.2.3.4"},
		{name: "zero/invalid ports fall back to 0", val: []any{"1.2.3.4", float64(0), "notaport"}, wantIP: "1.2.3.4"},
		{name: "empty array errors", val: []any{}, wantErr: true},
		{name: "nil errors", val: nil, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, sshPort, wgPort, err := parseNodeOutput(tt.val)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseNodeOutput() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if ip != tt.wantIP || sshPort != tt.wantSSHPort || wgPort != tt.wantWGPort {
				t.Errorf("parseNodeOutput() = (%q, %d, %d), want (%q, %d, %d)",
					ip, sshPort, wgPort, tt.wantIP, tt.wantSSHPort, tt.wantWGPort)
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
