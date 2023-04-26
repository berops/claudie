package utils

import (
	"bytes"

	"golang.org/x/crypto/blake2b"
)

// CalculateChecksum calculates a Blake2b 256 bit checksum of the passed data
// and returns it as a []byte.
func CalculateChecksum(data string) []byte {
	checksum := blake2b.Sum256([]byte(data))

	return checksum[:]
}

// CompareChecksum compares 2 checksums passed in as []byte and returns whether they are equal or not
func CompareChecksum(checksumA, checksumB []byte) bool {
	return bytes.Equal(checksumA, checksumB)
}
