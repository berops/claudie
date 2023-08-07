package usecases

const (
	// SpawnProcessLimit is the number of processes concurrently executing kubeone.
	SpawnProcessLimit = 5
)

type Usecases struct {
	// SpawnProcessLimit represents a synchronization channel which limits the number of spawned terraform
	// processes. This values must be non-nil and be buffered, where the capacity indicates
	// the limit.
	SpawnProcessLimit chan struct{}
}
