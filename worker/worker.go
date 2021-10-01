package worker

import (
	"context"
	"time"
)

// Worker struct
type Worker struct {
	tick         *time.Ticker
	fn           func() error
	errorHandler func(err error)
	ctx          context.Context
}

// NewWorker creates a new Worker structure
func NewWorker(ctx context.Context, d time.Duration, fn func() error, eh func(err error)) *Worker {
	return &Worker{
		ctx:          ctx,
		fn:           fn,
		errorHandler: eh,
		tick:         time.NewTicker(d),
	}
}

// Run starts the handling loop
func (w *Worker) Run() {
	for {
		select {
		case <-w.tick.C:
			if err := w.fn(); err != nil {
				if w.errorHandler != nil {
					w.errorHandler(err)
				}
			}
		case <-w.ctx.Done():
			w.tick.Stop()
			return
		}
	}
}
