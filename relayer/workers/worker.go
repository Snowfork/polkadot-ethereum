// Copyright 2021 Snowfork
// SPDX-License-Identifier: LGPL-3.0-only

package workers

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"
)

type Worker interface {
	Start(ctx context.Context, eg *errgroup.Group) error
}

type WorkerFactory func() (Worker, error)

type WorkerPool []WorkerFactory

func (wp WorkerPool) runWorker(ctx context.Context, worker Worker) error {
	childEg, childCtx := errgroup.WithContext(ctx)
	err := worker.Start(childCtx, childEg)
	if err != nil {
		return err
	}

	return childEg.Wait()
}

func (wp WorkerPool) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	eg, ctx := errgroup.WithContext(ctx)

	// Ensure clean termination upon SIGINT, SIGTERM
	eg.Go(func() error {
		notify := make(chan os.Signal, 1)
		signal.Notify(notify, syscall.SIGINT, syscall.SIGTERM)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-notify:
			// TODO: add logging back in
			//log.WithField("signal", sig.String()).Info("Received signal")
			cancel()
		}

		return nil
	})

	// TODO: add deadlock detection and warn devs

	for _, f := range wp {
		factory := f

		eg.Go(func() error {
			for {
				worker, err := factory()
				if err != nil {
					// It is unrecoverable if we cannot construct one of our workers
					return err
				}

				// TODO: log starting worker
				err = wp.runWorker(ctx, worker)
				// TODO: log ending worker

				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					// TODO: instead retry with backoff up to X retries
					return err
				}
			}
		})
	}

	return eg.Wait()
}
