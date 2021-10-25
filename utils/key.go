package utils

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

// CreateKeyFile writes the given key to a file.
// The key filename is specified by its outputPath and KeyName operands.
func CreateKeyFile(key string, outputPath string, keyName string) error {
	keyFileName := filepath.Join(outputPath, keyName)
	return ioutil.WriteFile(keyFileName, []byte(key), 0600)
}

// GetenvOr returns the value of the env variable argument if it exists.
// Otherwise it returns the provided default value.
func GetenvOr(envKey string, defaultVal string) string {
	v, present := os.LookupEnv(envKey)
	if present {
		return v
	} else {
		return defaultVal
	}
}
