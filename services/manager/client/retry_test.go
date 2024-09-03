package managerclient

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
)

func TestRetry(t *testing.T) {
	type args struct {
		logger      *zerolog.Logger
		description string
		fn          func() error
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok-on-3rd-retry",
			args: args{
				logger:      func() *zerolog.Logger { k := zerolog.New(new(bytes.Buffer)); return &k }(),
				description: "testing",
				fn: func() func() error {
					retry := 3
					return func() error {
						retry--
						if retry == 0 {
							return nil
						}
						return ErrVersionMismatch
					}
				}(),
			},
			wantErr: false,
		},
		{
			name: "fail-all-retries",
			args: args{
				logger:      func() *zerolog.Logger { k := zerolog.New(new(bytes.Buffer)); return &k }(),
				description: "testing",
				fn: func() func() error {
					retry := TolerateDirtyWrites
					return func() error {
						if retry == 0 {
							return nil
						}
						retry--
						return ErrVersionMismatch
					}
				}(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := Retry(tt.args.logger, tt.args.description, tt.args.fn); (err != nil) != tt.wantErr {
				t.Errorf("Retry() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
