package clusters

import (
	"errors"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func FuzzPingAll(f *testing.F) {
	testcases := []string{
		"1,2,3",
		"1,2,3,4,5",
		"1,2,3,4,5,6,7,8,9,10",
		"1,2,3,4,5,6,7,8,9,16,17,18",
	}
	for _, tc := range testcases {
		f.Add(tc)
	}
	log := zerolog.Logger{}

	f.Fuzz(func(t *testing.T, s string) {
		ips := strings.Split(s, ",")
		gc := rand.IntN(20)
		u, err := pingAll(log, gc, ips, func(logger zerolog.Logger, count int, dst string) error { return nil })
		if err != nil {
			t.Errorf("pingAll() goroutines = %v, ips = %v, unreachable = %v, err = %v", gc, ips, u, err)
		}
	})
}

func TestPingAll(t *testing.T) {
	logger := zerolog.Logger{}
	type args struct {
		goroutineCount int
		ips            []string
		f              func(logger zerolog.Logger, count int, dst string) error
	}

	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			args: args{
				goroutineCount: 0,
				ips:            []string{},
				f:              func(logger zerolog.Logger, count int, dst string) error { return nil },
			},
		},
		{
			args: args{
				goroutineCount: 3,
				ips:            []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"},
				f:              func(logger zerolog.Logger, count int, dst string) error { return nil },
			},
			want:    nil,
			wantErr: false,
		},
		{
			args: args{
				goroutineCount: 3,
				ips:            []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"},
				f: func(logger zerolog.Logger, count int, dst string) error {
					switch dst {
					case "1", "2", "3":
						return errors.New("not reachable")
					}
					return nil
				},
			},
			want:    []string{"1", "2", "3"},
			wantErr: true,
		},
		{
			args: args{
				goroutineCount: 3,
				ips:            []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"},
				f: func(logger zerolog.Logger, count int, dst string) error {
					switch dst {
					case "1", "2", "6", "10":
						return errors.New("not reachable")
					}
					return nil
				},
			},
			want:    []string{"1", "2", "6", "10"},
			wantErr: true,
		},
		{
			args: args{
				goroutineCount: 3,
				ips:            []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
				f: func(logger zerolog.Logger, count int, dst string) error {
					switch dst {
					case "1", "2", "6", "10":
						return errors.New("not reachable")
					}
					return nil
				},
			},
			want:    []string{"1", "2", "6", "10"},
			wantErr: true,
		},
		{
			args: args{
				goroutineCount: 3,
				ips:            []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"},
				f: func(logger zerolog.Logger, count int, dst string) error {
					switch dst {
					case "1", "2", "6", "10":
						return errors.New("not reachable")
					}
					return nil
				},
			},
			want:    []string{"1", "2", "6", "10"},
			wantErr: true,
		},
		{
			args: args{
				goroutineCount: 3,
				ips:            []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19"},
				f: func(logger zerolog.Logger, count int, dst string) error {
					switch dst {
					case "1", "2", "6", "10":
						return errors.New("not reachable")
					}
					return nil
				},
			},
			want:    []string{"1", "2", "6", "10"},
			wantErr: true,
		},
		{
			args: args{
				goroutineCount: 3,
				ips:            []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19", "20", "21", "22", "23", "24", "25", "26", "27", "28"},
				f: func(logger zerolog.Logger, count int, dst string) error {
					switch dst {
					case "1", "2", "6", "10":
						return errors.New("not reachable")
					}
					return nil
				},
			},
			want:    []string{"1", "2", "6", "10"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := pingAll(logger, tt.args.goroutineCount, tt.args.ips, tt.args.f)
			if (gotErr != nil) != tt.wantErr {
				t.Fatalf("pingAll() got %v, want %v", gotErr, tt.wantErr)
				return
			}

			for _, got := range got {
				found := false
				for _, want := range tt.want {
					if want == got {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("pingAll() got %v missing in want %v", got, tt.want)
				}
			}
		})
	}
}
