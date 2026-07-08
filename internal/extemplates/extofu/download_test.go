package extofu

import (
	"bytes"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/berops/claudie/proto/pb/spec"
	"github.com/stretchr/testify/assert"
)

func mustParse(u *url.URL, err error) *url.URL {
	if err != nil {
		panic(err)
	}
	return u
}

func TestDownloadProviderUpdate(t *testing.T) {
	downloadDir := "./test2"
	t.Cleanup(func() { os.RemoveAll(downloadDir) })

	var (
		provider = &spec.Provider{
			Templates: &spec.TemplateRepository{
				Endpoint: &spec.TemplateRepository_Endpoint{
					Url:      "github.com/berops/claudie-config",
					Protocol: spec.TemplateRepository_Endpoint_PROTOCOL_HTTPS,
				},
				Commit:     "v0.1.2",
				CommitHash: "42e963e4bcaa5cbf7ce3330c1b7a21ebaa30f79b",
				Paths: &spec.TemplateRepository_TemplatePaths{
					Terraformer: "/templates/terraformer",
				},
			},
		}
		repoURL      = mustParse(url.Parse(provider.Templates.HttpsUrl()))
		gitDirectory = filepath.Join(downloadDir, repoURL.Hostname(), repoURL.Path, "42e963e4bcaa5cbf7ce3330c1b7a21ebaa30f79b")
	)

	if err := Download(downloadDir, provider); err != nil {
		t.Errorf("DownloadProvider() error = %v", err)
	}

	out := new(bytes.Buffer)
	//nolint
	gitCmd := exec.Command("git", "rev-parse", "HEAD")
	gitCmd.Dir = gitDirectory
	gitCmd.Stdout = out
	if err := gitCmd.Run(); err != nil {
		t.Fatalf("failed to execute git checkout %v", err.Error())
	}
	assert.Equal(t, strings.TrimSpace(out.String()), "42e963e4bcaa5cbf7ce3330c1b7a21ebaa30f79b")
	out.Reset()

	//nolint
	gitCmd = exec.Command("git", "checkout", "74d4c23d5eb6c04cd4197be177989dce3a512981")
	gitCmd.Dir = gitDirectory
	if err := gitCmd.Run(); err != nil {
		t.Fatalf("failed to execute git checkout %v", err.Error())
	}

	if err := Download(downloadDir, provider); err != nil {
		t.Errorf("DownloadProvider() error = %v", err)
	}

	shouldExist := filepath.Join(gitDirectory, provider.Templates.Paths.Terraformer, "gcp")
	if _, err := os.Stat(shouldExist); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed to check existence of %q: %v", shouldExist, err)
		}
		t.Fatal(err)
	}

	//nolint
	gitCmd = exec.Command("git", "rev-parse", "HEAD")
	gitCmd.Dir = gitDirectory
	gitCmd.Stdout = out
	if err := gitCmd.Run(); err != nil {
		t.Fatalf("failed to execute git checkout %v", err.Error())
	}
	assert.Equal(t, strings.TrimSpace(out.String()), "42e963e4bcaa5cbf7ce3330c1b7a21ebaa30f79b")
	out.Reset()
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
						Endpoint: &spec.TemplateRepository_Endpoint{
							Url:      "github.com/berops/claudie-config",
							Protocol: spec.TemplateRepository_Endpoint_PROTOCOL_HTTPS,
						},
						Paths: &spec.TemplateRepository_TemplatePaths{
							Terraformer: "/templates/terraformer/gcp",
						},
						Commit:     "aa7bd5cfa382f8030494766016c59e8a2034cfcd",
						CommitHash: "aa7bd5cfa382f8030494766016c59e8a2034cfcd",
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
						Endpoint: &spec.TemplateRepository_Endpoint{
							Url:      "github.com/berops/claudie-config",
							Protocol: spec.TemplateRepository_Endpoint_PROTOCOL_HTTPS,
						},
						Commit: "v0.1.0",
						Paths: &spec.TemplateRepository_TemplatePaths{
							Terraformer: "/templates/gcp",
						},
						CommitHash: "ed25f730d859489aa994f75811ec90688aa1b82d",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "test-03",
			args: args{
				downloadInto: downloadDir,
				provider: &spec.Provider{
					Templates: &spec.TemplateRepository{
						Endpoint: &spec.TemplateRepository_Endpoint{
							Url:      "github.com/berops/claudie-config",
							Protocol: spec.TemplateRepository_Endpoint_PROTOCOL_HTTPS,
						},
						Commit: "0.0.0",
						Paths: &spec.TemplateRepository_TemplatePaths{
							Terraformer: "/templates",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "test-04",
			args: args{
				downloadInto: downloadDir,
				provider: &spec.Provider{
					Templates: &spec.TemplateRepository{
						Endpoint: &spec.TemplateRepository_Endpoint{
							Url:      "github.com/berops/claudie-config",
							Protocol: spec.TemplateRepository_Endpoint_PROTOCOL_UNSPECIFIED,
						},
						Commit: "v0.1.0",
						Paths: &spec.TemplateRepository_TemplatePaths{
							Terraformer: "/templates",
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Download(tt.args.downloadInto, tt.args.provider); (err != nil) != tt.wantErr {
				t.Errorf("DownloadProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
