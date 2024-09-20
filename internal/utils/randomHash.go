package utils

import (
	"math/rand"
	"strings"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyz" + "0123456789"

// HashLength is the length of the randomly generated "hash" that is used throughout claudie in different places.
// be CAUTIOUS when changing this value as this will break backwards compatibility and also invariants within claudie.
// Dynamic nodepools are assigned a randomly generated hash. Node pool names have a max constraint of 15 characters
// changing the hash length will invalidate this.
const HashLength = 7

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func CreateHash(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func MatchNameAndHashWithTemplate(template, nodepoolName string) (name, hash string) {
	if len(nodepoolName) != len(template)+HashLength+1 {
		return
	}

	idx := strings.LastIndex(nodepoolName, "-")
	if idx < 0 {
		return "", ""
	}

	if nodepoolName[:idx] != template {
		return
	}

	name = nodepoolName[:idx]
	hash = nodepoolName[idx+1:]

	return
}

func MustExtractNameAndHash(pool string) (name, hash string) {
	idx := strings.LastIndex(pool, "-")
	if idx < 0 {
		panic("this function expect that the nodepool name contains a appended hash delimited by '-'")
	}

	name = pool[:idx]
	hash = pool[idx+1:]

	return name, hash
}
