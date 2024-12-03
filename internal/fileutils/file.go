package fileutils

import (
	"os"
	"path/filepath"
)

// CreateKey writes the given key to a file.
// The key filename is specified by its outputPath and KeyName operands.
func CreateKey(key string, outputPath string, keyName string) error {
	keyFileName := filepath.Join(outputPath, keyName)
	return os.WriteFile(keyFileName, []byte(key), 0600)
}

func DirectoryExists(dir string) bool {
	_, err := os.Stat(dir)
	return err == nil
}

func CreateDirectory(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}
