package concurrent

import "golang.org/x/sync/errgroup"

func Exec[K any](items []K, f func(index int, item K) error) error {
	group := errgroup.Group{}

	for i, item := range items {
		group.Go(func() error {
			return f(i, item)
		})
	}

	return group.Wait()
}
