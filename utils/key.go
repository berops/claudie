package utils

import (
	"io/ioutil"
)

func CreateKeyFile(key string, outputPath string, keyName string) error {
	return ioutil.WriteFile(outputPath+keyName, []byte(key), 0600)
}
