package checksum

import (
	"bytes"

	"golang.org/x/crypto/blake2b"
)

func CalculateChecksum(data string) []byte {
	res := blake2b.Sum256([]byte(data))
	// Creating a slice using an array you can just make a simple slice expression
	return res[:]
}

func CompareChecksums(ch1, ch2 []byte) bool {
	return bytes.Equal(ch1, ch2)
}
