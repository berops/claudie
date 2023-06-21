package utils

import (
	"golang.org/x/sync/errgroup"
)

func ConcurrentExec[K any](items []K, f func(index int, item K) error) error {
	group := errgroup.Group{}

	for i, item := range items {
		func(index int, item K) {
			group.Go(func() error {
				return f(index, item)
			})
		}(i, item)
	}

	return group.Wait()
}
