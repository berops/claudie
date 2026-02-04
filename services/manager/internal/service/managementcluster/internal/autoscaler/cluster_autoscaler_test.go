package autoscaler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getK8sVersion(t *testing.T) {
	out, err := getK8sVersion("v1.29.4")
	assert.Nil(t, err)
	assert.Equal(t, "v1.29.5", out)

	out, err = getK8sVersion("1.29.4")
	assert.Nil(t, err)
	assert.Equal(t, "v1.29.5", out)
}
