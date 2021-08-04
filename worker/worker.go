package worker

import (
	"context"
	"time"
)

type Worker struct {
	tick         *time.Ticker
	fn           func() error
	errorHandler func(err error)
	ctx          context.Context
}

func NewWorker(d time.Duration, ctx context.Context, fn func() error, eh func(err error)) *Worker {
	return &Worker{
		ctx:          ctx,
		fn:           fn,
		errorHandler: eh,
		tick:         time.NewTicker(d),
	}
}

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
		}
	}
}
