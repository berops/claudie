package utils

import (
	"os"
)

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
