package copy

import (
	"io"
	"os"
	"path/filepath"
)

// file copies a single file from src to dst
func file(src, dst string) error {
	var err error
	var srcfd *os.File
	var dstfd *os.File
	var srcinfo os.FileInfo

	if srcfd, err = os.Open(src); err != nil {
		return err
	}
	defer srcfd.Close()

	if dstfd, err = os.Create(dst); err != nil {
		return err
	}
	defer dstfd.Close()

	if _, err = io.Copy(dstfd, srcfd); err != nil {
		return err
	}
	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}
	return os.Chmod(dst, srcinfo.Mode())
}

func FileToDir(srcFile, dstDir string) error {
	var err error
	var srcfd *os.File
	var dstfd *os.File
	var srcinfo os.FileInfo

	if srcfd, err = os.Open(srcFile); err != nil {
		return err
	}
	defer srcfd.Close()

	if info, err := os.Stat(dstDir); os.IsNotExist(err) {
		if err = os.MkdirAll(dstDir, info.Mode()); err != nil {
			return err
		}
	}

	if dstfd, err = os.Create(filepath.Join(dstDir, srcFile)); err != nil {
		return err
	}
	defer dstfd.Close()

	if _, err = io.Copy(dstfd, srcfd); err != nil {
		return err
	}
	if srcinfo, err = os.Stat(srcFile); err != nil {
		return err
	}
	return os.Chmod(filepath.Join(dstDir, srcFile), srcinfo.Mode())
}
