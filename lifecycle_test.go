package worker_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	worker "github.com/fin-assist/go-worker"
)

func TestIntegrationRealClockRestartsAndJoinsIndependentJobs(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cleaned := make(chan struct{})
	release := make(chan struct{})
	started := make(chan struct{})
	restarted := make(chan string, 3)
	jobs := []worker.Job{{Name: "cleanup", Run: func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		<-release
		close(cleaned)
		return ctx.Err()
	}}}
	for _, mode := range []string{"nil", "error", "panic"} {
		var n atomic.Int32
		jobs = append(jobs, worker.Job{Name: mode, Run: func(ctx context.Context) error {
			if n.Add(1) == 1 {
				if mode == "error" {
					return errors.New("failure")
				}
				if mode == "panic" {
					panic("failure")
				}
				return nil
			}
			restarted <- mode
			<-ctx.Done()
			return ctx.Err()
		}})
	}
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx, options(), jobs...) }()
	receive(t, started)
	for range 3 {
		receive(t, restarted)
	}
	cancel()
	select {
	case err := <-done:
		t.Fatalf("supervisor returned before job cleanup: %v", err)
	default:
	}
	close(release)
	receive(t, cleaned)
	if !errors.Is(receive(t, done), context.Canceled) {
		t.Fatal("shutdown")
	}
}
func TestIntegrationRealScheduleContinuesAfterError(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var n atomic.Int32
	job, err := worker.Every("periodic", time.Millisecond, logger(), worker.SystemClock{}, func(context.Context) error {
		if n.Add(1) == 3 {
			cancel()
		}
		return errors.New("iteration")
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.Run(ctx, options(), job); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if n.Load() < 3 {
		t.Fatal("schedule stopped after error")
	}
}
func TestIntegrationInvalidAssemblyStartsNothing(t *testing.T) { checkInvalidAssembly(t) }
func checkInvalidAssembly(t *testing.T) {
	t.Helper()
	if worker.Run(t.Context(), worker.Options{}) == nil {
		t.Fatal("missing dependencies")
	}
	if worker.Run(t.Context(), options(), worker.Job{}) == nil {
		t.Fatal("invalid job")
	}
	if _, err := worker.Every("bad", 0, logger(), worker.SystemClock{}, func(context.Context) error { return nil }); err == nil {
		t.Fatal("invalid schedule")
	}
	if err := worker.Run(t.Context(), options()); err != nil {
		t.Fatal(err)
	}
}
