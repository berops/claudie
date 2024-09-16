package checksum

import (
	"golang.org/x/crypto/blake2b"
)

// Digest calculates a Blake2b 256 bit checksum of the passed data.
func Digest(data string) []byte {
	checksum := blake2b.Sum256([]byte(data))
	return checksum[:]
}
