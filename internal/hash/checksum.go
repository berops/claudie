package hash

import (
	"crypto/sha512"
	"math/rand"
	"time"
)

const (
	charset = "abcdefghijklmnopqrstuvwxyz" + "0123456789"

	// Length is the length of the randomly generated "hash" that is used throughout claudie in different places.
	// be CAUTIOUS when changing this value as this will break backwards compatibility and also invariants within claudie.
	// Dynamic nodepools are assigned a randomly generated hash. Node pool names have a max constraint of 15 characters
	// changing the hash length will invalidate this.
	Length = 7
)

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func Digest(data string) []byte {
	digest := sha512.Sum512_256([]byte(data))
	return digest[:]
}

func Digest128(data string) []byte { return Digest(data)[:16] }

func Create(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
