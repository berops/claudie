package utils // import "github.com/Berops/platform/utils"

import (
	"fmt"
	"os"
	"path/filepath"
)

// DeleteTmpFiles deletes a set of files as specified in parameters.
// First operand filePath is the directory containing the temporary files.
// Second operand tmpFiles is an array (not vararg) of fileNames to be deleted.
func DeleteTmpFiles(filePath string, tmpFiles []string) error {
	for _, fileName := range tmpFiles {
		tmpFilePath := filepath.Join(filePath, fileName)
		if err := os.Remove(tmpFilePath); err != nil {
			return fmt.Errorf("error while deleting %s file: %v", tmpFilePath, err)
		}
	}

	return nil
}
