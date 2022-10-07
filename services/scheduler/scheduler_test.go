package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakeSSHKeyPair(t *testing.T) {
	pair, err := makeSSHKeyPair()
	require.NoError(t, err)
	t.Log(pair.private)
	t.Log(pair.public)
}
