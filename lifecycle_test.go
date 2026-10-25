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
	// The third call cancels: fewer calls means errors stopped the schedule, more means
	// an iteration started after cancellation (FIN-265).
	if got := n.Load(); got != 3 {
		t.Fatalf("want exactly 3 iterations, got %d", got)
	}
}

func TestIntegrationRealScheduleStopsOnCancellationWithPendingTick(t *testing.T) {
	const interval = time.Millisecond
	// Without the post-tick context check select picks the pending tick about half of
	// the time; repetitions make a regression fail with near certainty.
	for range 50 {
		ctx, cancel := context.WithCancel(t.Context())
		var calls atomic.Int32
		job, err := worker.Every("slow-real", interval, logger(), worker.SystemClock{}, func(ctx context.Context) error {
			if calls.Add(1) == 1 {
				// Real time passes beyond the interval while the callback is held, so the
				// stdlib ticker queues a tick before cancellation; no goroutine handoff
				// depends on this wait.
				<-time.After(20 * interval)
				cancel()
			}
			return ctx.Err()
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := worker.Run(ctx, options(), job); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if got := calls.Load(); got != 1 {
			t.Fatalf("iteration started after cancellation: %d calls", got)
		}
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
