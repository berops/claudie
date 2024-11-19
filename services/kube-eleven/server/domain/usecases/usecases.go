package usecases

import "golang.org/x/sync/semaphore"

const (
	// SpawnProcessLimit is the number of processes concurrently executing kubeone.
	SpawnProcessLimit = 5
)

type Usecases struct {
	// SpawnProcessLimit limits the number of spawned terraform processes.
	SpawnProcessLimit *semaphore.Weighted
}
