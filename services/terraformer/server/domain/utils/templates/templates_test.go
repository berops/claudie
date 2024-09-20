package templates_test

import (
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/templates"
)

func strPtr(s string) *string { return &s }

func mustParse(u *url.URL, err error) *url.URL {
	if err != nil {
		panic(err)
	}
	return u
}

func TestDownloadProviderUpdate(t *testing.T) {
	downloadDir := "./test2"
	t.Cleanup(func() { os.RemoveAll(downloadDir) })

	var provider = &spec.Provider{
		Templates: &spec.TemplateRepository{
			Repository: "https://github.com/berops/claudie-config",
			Tag:        strPtr("v0.1.2"),
			Path:       "/templates/terraformer/gcp",
			CommitHash: "42e963e4bcaa5cbf7ce3330c1b7a21ebaa30f79b",
		},
	}

	if err := templates.DownloadProvider(downloadDir, provider); err != nil {
		t.Errorf("DownloadProvider() error = %v", err)
	}

	repoURL := mustParse(url.Parse(provider.Templates.Repository))

	gitDirectory := filepath.Join(downloadDir, repoURL.Hostname(), repoURL.Path, "42e963e4bcaa5cbf7ce3330c1b7a21ebaa30f79b")
	gitCmd := exec.Command("git", "checkout", "74d4c23d5eb6c04cd4197be177989dce3a512981")
	gitCmd.Dir = gitDirectory
	if err := gitCmd.Run(); err != nil {
		t.Fatalf("failed to execute git checkout %v", err.Error())
	}

	if err := templates.DownloadProvider(downloadDir, provider); err != nil {
		t.Errorf("DownloadProvider() error = %v", err)
	}

	shouldExist := filepath.Join(gitDirectory, provider.Templates.Path)
	if _, err := os.Stat(shouldExist); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed to check existence of %q: %v", shouldExist, err)
		}
		t.Fatal(err)
	}
}

func TestDownloadProvider(t *testing.T) {
	downloadDir := "./test"
	t.Cleanup(func() { os.RemoveAll(downloadDir) })

	type args struct {
		downloadInto string
		provider     *spec.Provider
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test-01",
			args: args{
				downloadInto: downloadDir,
				provider: &spec.Provider{
					Templates: &spec.TemplateRepository{
						Repository: "https://github.com/berops/claudie-config",
						Path:       "/templates/terraformer/gcp",
						CommitHash: "aa7bd5cfa382f8030494766016c59e8a2034cfcd",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "test-01",
			args: args{
				downloadInto: downloadDir,
				provider: &spec.Provider{
					Templates: &spec.TemplateRepository{
						Repository: "https://github.com/berops/claudie-config",
						Tag:        strPtr("v0.1.0"),
						Path:       "/templates/gcp",
						CommitHash: "ed25f730d859489aa994f75811ec90688aa1b82d",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "test-02",
			args: args{
				downloadInto: downloadDir,
				provider: &spec.Provider{
					Templates: &spec.TemplateRepository{
						Repository: "https://github.com/berops/claudie-config",
						Tag:        strPtr("v0.0.0"),
						Path:       "/templates",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "test-03",
			args: args{
				downloadInto: downloadDir,
				provider: &spec.Provider{
					Templates: &spec.TemplateRepository{
						Repository: "h??ttps:/github.com/berops/claudie-config",
						Tag:        strPtr("v0.1.0"),
						Path:       "/templates",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := templates.DownloadProvider(tt.args.downloadInto, tt.args.provider); (err != nil) != tt.wantErr {
				t.Errorf("DownloadProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
