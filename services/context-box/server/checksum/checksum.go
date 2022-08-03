package checksum

import (
	"bytes"
	"crypto/md5"
)

func CalculateChecksum(data string) []byte {
	res := md5.Sum([]byte(data))
	// Creating a slice using an array you can just make a simple slice expression
	return res[:]
}

func CompareChecksums(ch1, ch2 []byte) bool {
	return bytes.Equal(ch1, ch2)
}
