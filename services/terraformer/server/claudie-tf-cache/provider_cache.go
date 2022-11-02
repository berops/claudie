package claudietfcache

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/Berops/claudie/internal/copy"
	"github.com/Berops/claudie/services/terraformer/server/terraform"
)

const (
	baseCacheDir = "services/terraformer/server/claudie-tf-cache"
	registry     = "registry.terraform.io"
)

var (
	lock = sync.Mutex{}
)

func InitProvidersPluginCache() error {
	tf := terraform.Terraform{Directory: baseCacheDir}
	if err := tf.TerraformProvidersMirror("."); err != nil {
		return err
	}
	return nil
}

func CopyCache(dst string) error {
	lock.Lock()
	defer lock.Unlock()
	if err := copy.DirToDir(filepath.Join(baseCacheDir, registry), filepath.Join(dst, registry)); err != nil {
		return fmt.Errorf("error while copying the terraform cache: %w", err)
	}
	return nil
}
