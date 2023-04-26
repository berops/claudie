package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	q = Queue{}
)

type subConfig struct {
	Name         string
	MsChecksum   []byte
	CsChecksum   []byte
	DsChecksum   []byte
	BuilderTTL   int32
	SchedulerTTL int32
	ErrorMessage string
}

// each element of the queue should have a GetName method to evaluate equivalence
func (subConfig *subConfig) GetName() string {
	return subConfig.Name
}

func TestQueue(t *testing.T) {
	q.Enqueue(&subConfig{Name: "foo"})
	q.Enqueue(&subConfig{Name: "bar"})
	require.EqualValues(t, 2, len(q.elements))
	c := q.Dequeue()
	require.EqualValues(t, 1, len(q.elements))
	require.EqualValues(t, "foo", c.GetName())
	require.True(t, q.Contains(&subConfig{Name: "bar"}))
}
