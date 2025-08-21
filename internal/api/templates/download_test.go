package templates_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/berops/claudie/internal/api/templates"
	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
)

func TestDownload(t *testing.T) {
	t.Parallel()

	downloadDir := "./test3"
	t.Cleanup(func() { os.RemoveAll(downloadDir) })

	ctx := context.Background()
	err := templates.Download(ctx, downloadDir, templates.Repository{
		Repository: "https://github.com/berops/claudie-config",
		Path:       "/templates/terraformer/gcp",
		CommitHash: "42e963e4bcaa5cbf7ce3330c1b7a21ebaa30f79b", // v0.1.2
	})
	assert.Nil(t, err)
}

func TestDownloadExisting(t *testing.T) {
	t.Parallel()

	downloadDir := "./test4"
	t.Cleanup(func() { os.RemoveAll(downloadDir) })

	// initial download
	ctx := context.Background()

	err := templates.Download(ctx, downloadDir, templates.Repository{
		Repository: "https://github.com/berops/claudie-config",
		Path:       "/templates/terraformer/gcp",
		CommitHash: "42e963e4bcaa5cbf7ce3330c1b7a21ebaa30f79b", // v0.1.2
	})
	assert.Nil(t, err, "failed to download git repository")

	targetDir := filepath.Join(downloadDir, "github.com", "berops", "claudie-config", "42e963e4bcaa5cbf7ce3330c1b7a21ebaa30f79b")
	mirror, err := git.PlainOpen(targetDir)
	assert.Nil(t, err, "git repository does not exists locally")

	ref, err := mirror.Head()
	assert.Nil(t, err, "failed to check local git repository")
	assert.Equal(t, "42e963e4bcaa5cbf7ce3330c1b7a21ebaa30f79b", ref.Hash().String(), "should match")

	// existing download
	err = templates.Download(ctx, downloadDir, templates.Repository{
		Repository: "https://github.com/berops/claudie-config",
		Path:       "/templates/terraformer/gcp",
		CommitHash: "42e963e4bcaa5cbf7ce3330c1b7a21ebaa30f79b", // v0.1.2
	})
	assert.Nil(t, err, "checkout local copy of the git repository")
}
