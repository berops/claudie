package utils

import (
	"math/rand"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyz" + "0123456789"
const HashLength = 7

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func CreateHash(length int) string {
	return CreateHashWithCharSet(length, charset)
}

func CreateHashWithCharSet(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
