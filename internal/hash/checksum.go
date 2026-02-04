package hash

import (
	"crypto/sha512"
	"math/rand/v2"
	"sync"
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

type generator struct {
	l sync.Mutex
	g *rand.Rand
}

func (g *generator) Intn(n int) int {
	g.l.Lock()
	defer g.l.Unlock()
	return g.g.IntN(n)
}

var rng = generator{
	g: rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), uint64(time.Now().Unix()+1))),
}

func Digest(data string) []byte {
	digest := sha512.Sum512_256([]byte(data))
	return digest[:]
}

func Digest128(data string) []byte { return Digest(data)[:16] }

// Create uses a pseudo-random-generator that is not cryptographically safe for generating the requested hash length
// using the specified [charset].
func Create(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rng.Intn(len(charset))]
	}
	return string(b)
}
