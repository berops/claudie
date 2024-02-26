package utils

import (
	"math/rand"
	"strings"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyz" + "0123456789"
const HashLength = 7

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func CreateHash(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func ExtractHashFromNodePool(template, nodepoolName string) (name, hash string) {
	if len(nodepoolName) != len(template)+HashLength+1 {
		return
	}

	idx := strings.LastIndex(nodepoolName, "-")
	if idx < 0 {
		panic("this function expect that the nodepool name contains a appended hash delimited by '-'")
	}

	if nodepoolName[:idx] != template {
		return
	}

	name = nodepoolName[:idx]
	hash = nodepoolName[idx+1:]

	return
}
