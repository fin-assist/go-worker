package worker_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	worker "github.com/fin-assist/go-worker"
)

// A service runs a scheduled partition check and three independent recoverable consumers.
func TestAPIServiceOwnsShutdownAndIndependentRecovery(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var logs bytes.Buffer
	opts := worker.Options{Logger: slog.New(slog.NewTextHandler(&logs, nil)), Clock: worker.SystemClock{}, RestartDelay: time.Millisecond}
	ticks := make(chan struct{}, 4)
	var iterations atomic.Int32
	scheduled, err := worker.Every("partition-check", time.Millisecond, opts.Logger, opts.Clock, func(context.Context) error {
		if iterations.Add(1) <= 3 {
			ticks <- struct{}{}
		}
		return errors.New("secret-financial-data")
	})
	if err != nil {
		t.Fatal(err)
	}
	recovered := make(chan string, 3)
	cleanup := make(chan struct{}, 3)
	jobs := []worker.Job{scheduled}
	for _, failure := range []string{"returned", "error", "panic"} {
		var calls atomic.Int32
		jobs = append(jobs, worker.Job{Name: failure, Run: func(ctx context.Context) error {
			if calls.Add(1) == 1 {
				switch failure {
				case "error":
					return errors.New("secret-token")
				case "panic":
					panic("secret-token")
				default:
					return nil
				}
			}
			recovered <- failure
			<-ctx.Done()
			cleanup <- struct{}{}
			return nil
		}})
	}
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx, opts, jobs...) }()
	for range 3 {
		receive(t, recovered)
	}
	for range 3 {
		receive(t, ticks)
	}
	cancel()
	if !errors.Is(receive(t, done), context.Canceled) {
		t.Fatal("service cancellation not preserved")
	}
	for range 3 {
		receive(t, cleanup)
	}
	if strings.Contains(logs.String(), "secret-") {
		t.Fatal("job payload escaped into process logs")
	}
	checkInvalidAssembly(t)
}
