package utils // import "github.com/Berops/platform/utils"

import (
	"io/ioutil"
	"path/filepath"
)

// CreateKeyFile writes the given key to a file.
// The key filename is specified by its outputPath and KeyName operands.
func CreateKeyFile(key string, outputPath string, keyName string) error {
	keyFileName := filepath.Join(outputPath, keyName)
	return ioutil.WriteFile(keyFileName, []byte(key), 0600)
}
