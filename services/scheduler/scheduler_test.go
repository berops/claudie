package main

import (
	"testing"
)

func TestMakeSSHKeyPair(t *testing.T) {
	priv, pub := makeSSHKeyPair()
	t.Log(priv)
	t.Log(pub)
}
