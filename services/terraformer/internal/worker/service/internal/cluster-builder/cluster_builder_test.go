package cluster_builder

import (
	"testing"
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
		{name: "port with suffix is rejected", val: []any{"1.2.3.4", "22222x", "41234x"}, wantIP: "1.2.3.4"},
		{name: "out-of-range ports fall back to 0", val: []any{"1.2.3.4", float64(65536), float64(99999)}, wantIP: "1.2.3.4"},
		{name: "non-string ip element errors", val: []any{float64(1234), "22222"}, wantErr: true},
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
