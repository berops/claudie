package nodepools

import (
	"testing"

	"github.com/berops/claudie/proto/pb/spec"
)

func TestNodeSSHPort(t *testing.T) {
	tests := []struct {
		name        string
		nodePoolSSH int32
		nodeSSH     int32
		want        int32
	}{
		{name: "both unset falls back to default 22", nodePoolSSH: 0, nodeSSH: 0, want: DefaultSSHPort},
		{name: "node pool port used when node unset", nodePoolSSH: 2222, nodeSSH: 0, want: 2222},
		{name: "per-node port overrides node pool", nodePoolSSH: 2222, nodeSSH: 22222, want: 22222},
		{name: "per-node port overrides default", nodePoolSSH: 0, nodeSSH: 22222, want: 22222},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			np := &spec.NodePool{SshPort: tt.nodePoolSSH}
			n := &spec.Node{SshPort: tt.nodeSSH}
			if got := NodeSSHPort(np, n); got != tt.want {
				t.Errorf("NodeSSHPort() = %d, want %d", got, tt.want)
			}
		})
	}
}
