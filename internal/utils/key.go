package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/berops/claudie/proto/pb"
)

// CreateKeyFile writes the given key to a file.
// The key filename is specified by its outputPath and KeyName operands.
func CreateKeyFile(key string, outputPath string, keyName string) error {
	keyFileName := filepath.Join(outputPath, keyName)
	return os.WriteFile(keyFileName, []byte(key), 0600)
}

// CreateKeysForStaticNodepools creates private keys files for all nodes in the provided static node pools in form
// of <node name>.pem.
func CreateKeysForStaticNodepools(nps []*pb.NodePool, outputDirectory string) error {
	errs := make([]error, 0, len(nps))
	for _, staticNp := range nps {
		for _, node := range staticNp.Nodes {
			if key, ok := staticNp.GetStaticNodePool().NodeKeys[node.Public]; ok {
				if err := CreateKeyFile(key, outputDirectory, fmt.Sprintf("%s.pem", node.Name)); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}
	// If empty, returns nil
	return errors.Join(errs...)
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
