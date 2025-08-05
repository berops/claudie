package templates_test

import (
	"testing"

	"github.com/berops/claudie/internal/api/templates"
)

func TestDownloadProviderUpdate(t *testing.T) {
	// TODO: finish me.
	downloadDir := "./test3"
	// t.Cleanup(func() { os.RemoveAll(downloadDir) })

	err := templates.Download(downloadDir, templates.Repository{
		// TODO: pattern match on supported protocol currently should be just https.
		Repository: "https://github.com/berops/claudie-config",
		Path:       "/templates/terraformer/gcp",
		Commit:     "v0.1.2",
		CommitHash: "42e963e4bcaa5cbf7ce3330c1b7a21ebaa30f79b",
	})
	if err != nil {
		t.Fatal(err)
	}
}
