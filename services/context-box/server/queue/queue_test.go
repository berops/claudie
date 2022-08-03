package queue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	q = Queue{}
)

type ci struct {
	Name         string
	MsChecksum   []byte
	CsChecksum   []byte
	DsChecksum   []byte
	BuilderTTL   int32
	SchedulerTTL int32
	ErrorMessage string
}

// GetName is function required by queue package to evaluate equivalence
func (ci *ci) GetName() string {
	return ci.Name
}
func TestQueue(t *testing.T) {
	q.Enqueue(&ci{Name: "foo"})
	q.Enqueue(&ci{Name: "bar"})
	require.EqualValues(t, 2, len(q.queue))
	c := q.Dequeue()
	require.EqualValues(t, 1, len(q.queue))
	require.EqualValues(t, "foo", c.GetName())
	require.True(t, q.Contains(&ci{Name: "bar"}))
}
