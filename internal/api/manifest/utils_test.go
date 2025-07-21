package manifest

import (
	"github.com/berops/claudie/proto/pb/spec"
	"testing"
)

func Test_commitHash(t *testing.T) {
	type args struct {
		tmpl *spec.TemplateRepository
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "ok-tag-0.8.1",
			args: args{
				tmpl: &spec.TemplateRepository{
					Repository: "https://github.com/berops/claudie",
					Tag:        func() *string { s := "v0.8.1"; return &s }(),
				},
			},
			want:    "dc323eb49b5023306a5a70789d5a192f68e0a3a1",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FetchCommitHash(tt.args.tmpl)
			if (err != nil) != tt.wantErr {
				t.Errorf("commitHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.args.tmpl.CommitHash != tt.want {
				t.Errorf("commitHash() got = %v, want %v", tt.args.tmpl.CommitHash, tt.want)
			}
		})
	}
}
