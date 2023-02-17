package utils

import (
	"golang.org/x/sync/errgroup"
)

func ConcurrentExec[K any](items []K, f func(item K) error) error {
	group := errgroup.Group{}

	for _, item := range items {
		func(item K) {
			group.Go(func() error {
				return f(item)
			})
		}(item)
	}

	return group.Wait()
}
