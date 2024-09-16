package managerclient

import (
	"bytes"
	"github.com/stretchr/testify/assert"
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
		name     string
		args     args
		validate func(t *testing.T, err error)
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
			validate: func(t *testing.T, err error) { assert.Nil(t, err) },
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
			validate: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrVersionMismatch)
				assert.ErrorIs(t, err, ErrRetriesExhausted)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Retry(tt.args.logger, tt.args.description, tt.args.fn)
			tt.validate(t, err)
		})
	}
}
