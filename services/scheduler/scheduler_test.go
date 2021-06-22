package main

import (
	"testing"
)

func TestMakeSSHKeyPair(t *testing.T) {
	priv, pub := MakeSSHKeyPair()
	t.Log(priv)
	t.Log(pub)
}
