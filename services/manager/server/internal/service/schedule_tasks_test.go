package service

import (
	"testing"

	"github.com/berops/claudie/proto/pb/spec"
	"github.com/stretchr/testify/assert"
)

func Test_findNewAPIEndpointCandidate(t *testing.T) {
	type args struct {
		desired []*spec.NodePool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "find-candidate-ok",
			args: args{
				desired: []*spec.NodePool{
					{Name: "np-0", IsControl: false},
					{Name: "np-1", IsControl: true},
				},
			},
			want: "np-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, findNewAPIEndpointCandidate(tt.args.desired), "findNewAPIEndpointCandidate(%v)", tt.args.desired)
		})
	}
}
