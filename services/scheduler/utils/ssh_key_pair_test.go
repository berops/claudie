package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakeSSHKeyPair(t *testing.T) {
	pair, err := generateSSHKeyPair()
	require.NoError(t, err)
	t.Log(pair.private)
	t.Log(pair.public)
}
