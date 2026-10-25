// Package worker supervises cooperative background jobs in a service process.
package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// Job is a named loop. Name must be bounded operator configuration, not user data.
// Run must honor cancellation and finish its own goroutines
// before returning. Jobs run independently; failures restart after a pause.
type Job struct {
	Name string
	Run  func(context.Context) error
}

// Options supplies process-owned dependencies. RestartDelay zero means five seconds;
// negative delays are invalid. Clock and Logger must be non-nil and concurrency safe.
type Options struct {
	Logger       *slog.Logger
	Clock        Clock
	RestartDelay time.Duration
}

// Run validates all jobs before starting any, runs them concurrently, and waits for
// every job to return after cancellation. It returns ctx.Err() after all jobs finish
// (nil for no jobs on a live context). An error, premature nil return or panic
// restarts only that job. Cancellation cannot forcibly stop a job that ignores its context.
// Run is safe for concurrent calls with independent jobs and shared safe dependencies.
func Run(ctx context.Context, o Options, jobs ...Job) error {
	if o.Logger == nil || o.Clock == nil || o.RestartDelay < 0 {
		return errors.New("worker: logger, clock and nonnegative restart delay required")
	}
	for _, j := range jobs {
		if j.Name == "" || j.Run == nil {
			return errors.New("worker: job name and function required")
		}
	}
	if o.RestartDelay == 0 {
		o.RestartDelay = 5 * time.Second
	}
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Go(func() {
			for ctx.Err() == nil {
				reason := runJob(ctx, j)
				if ctx.Err() != nil {
					return
				}
				// Job errors and panic payloads can contain secrets; record bounded reasons only.
				o.Logger.ErrorContext(ctx, "job stopped, restarting", "job", j.Name, "reason", reason, "delay", o.RestartDelay)
				timer := o.Clock.NewTimer(o.RestartDelay)
				select {
				case <-ctx.Done():
				case <-timer.C():
				}
				timer.Stop()
			}
		})
	}
	wg.Wait()
	return ctx.Err()
}

func runJob(ctx context.Context, j Job) (reason string) {
	defer func() {
		if recover() != nil {
			reason = "panic"
		}
	}()
	if j.Run(ctx) != nil {
		return "error"
	}
	return "returned"
}

// Every constructs a sequential scheduled job. Its first iteration starts immediately,
// including with an already cancelled context, as in the service worker it replaces.
// Later iterations follow a ticker started before that first call; slow calls coalesce
// ticks according to Clock. Iteration errors do not stop the schedule; panics reach Run
// for restart. fn must honor cancellation. No goroutine starts until Job.Run is called.
// A single returned job may run concurrently only if fn and dependencies permit it.
func Every(name string, interval time.Duration, logger *slog.Logger, clock Clock, fn func(context.Context) error) (Job, error) {
	if name == "" || interval <= 0 || logger == nil || clock == nil || fn == nil {
		return Job{}, errors.New("worker: name, positive interval, logger, clock and function required")
	}
	return Job{Name: name, Run: func(ctx context.Context) error {
		ticker := clock.NewTicker(interval)
		defer ticker.Stop()
		for {
			if err := fn(ctx); err != nil && ctx.Err() == nil {
				logger.ErrorContext(ctx, "job iteration failed", "job", name, "reason", "error")
			}
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C():
			}
		}
	}}, nil
}
