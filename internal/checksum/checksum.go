package checksum

import (
	"crypto/sha512"
)

func Digest(data string) []byte {
	digest := sha512.Sum512_256([]byte(data))
	return digest[:]
}

func Digest128(data string) []byte { return Digest(data)[:16] }
