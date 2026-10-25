package worker_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	worker "github.com/fin-assist/go-worker"
)

type signal struct {
	c       chan time.Time
	stopped chan struct{}
}

func (s *signal) C() <-chan time.Time { return s.c }
func (s *signal) Stop()               { close(s.stopped) }

type manualClock struct {
	timers  chan *signal
	tickers chan *signal
	delay   chan time.Duration
}

func newClock() *manualClock {
	return &manualClock{make(chan *signal, 10), make(chan *signal, 10), make(chan time.Duration, 10)}
}
func (c *manualClock) NewTimer(d time.Duration) worker.Timer {
	s := &signal{make(chan time.Time, 1), make(chan struct{})}
	c.delay <- d
	c.timers <- s
	return s
}
func (c *manualClock) NewTicker(d time.Duration) worker.Ticker {
	s := &signal{make(chan time.Time, 1), make(chan struct{})}
	c.delay <- d
	c.tickers <- s
	return s
}
func receive[T any](t *testing.T, c <-chan T) T {
	t.Helper()
	select {
	case v := <-c:
		return v
	case <-time.After(3 * time.Second):
		t.Fatal("deadline waiting for lifecycle signal")
		var zero T
		return zero
	}
}
func logger() *slog.Logger { return slog.New(slog.DiscardHandler) }
func options() worker.Options {
	return worker.Options{Logger: logger(), Clock: worker.SystemClock{}, RestartDelay: time.Millisecond}
}

func TestUnitValidationBeforeStartingJobs(t *testing.T) {
	called := false
	job := worker.Job{Name: "valid", Run: func(context.Context) error { called = true; return nil }}
	for _, o := range []worker.Options{{}, {Logger: logger()}, {Clock: worker.SystemClock{}}, {Logger: logger(), Clock: worker.SystemClock{}, RestartDelay: -1}} {
		if worker.Run(t.Context(), o, job) == nil {
			t.Fatal("invalid dependencies admitted")
		}
	}
	for _, bad := range []worker.Job{{Name: "missing"}, {Run: job.Run}} {
		if worker.Run(t.Context(), options(), job, bad) == nil {
			t.Fatal("partial batch started")
		}
	}
	if called {
		t.Fatal("validation ran a job")
	}
	if err := worker.Run(t.Context(), options()); err != nil {
		t.Fatal(err)
	}
	for _, args := range []struct {
		name     string
		interval time.Duration
		log      *slog.Logger
		clock    worker.Clock
		fn       func(context.Context) error
	}{
		{"", time.Second, logger(), worker.SystemClock{}, job.Run}, {"x", 0, logger(), worker.SystemClock{}, job.Run}, {"x", time.Second, nil, worker.SystemClock{}, job.Run}, {"x", time.Second, logger(), nil, job.Run}, {"x", time.Second, logger(), worker.SystemClock{}, nil},
	} {
		if _, err := worker.Every(args.name, args.interval, args.log, args.clock, args.fn); err == nil {
			t.Fatal("invalid schedule admitted")
		}
	}
}

func TestUnitRestartsHaveDefaultDelayStopTimersAndSanitizeLogs(t *testing.T) {
	for _, mode := range []string{"error", "nil", "panic"} {
		t.Run(mode, func(t *testing.T) {
			clock := newClock()
			var logs bytes.Buffer
			log := slog.New(slog.NewTextHandler(&logs, nil))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			calls := make(chan int, 4)
			var count atomic.Int32
			job := worker.Job{Name: "configured-job", Run: func(ctx context.Context) error {
				n := int(count.Add(1))
				calls <- n
				if n == 1 {
					switch mode {
					case "error":
						return errors.New("secret-token")
					case "panic":
						panic("secret-token")
					default:
						return nil
					}
				}
				<-ctx.Done()
				return ctx.Err()
			}}
			done := make(chan error, 1)
			go func() { done <- worker.Run(ctx, worker.Options{Logger: log, Clock: clock}, job) }()
			if receive(t, calls) != 1 {
				t.Fatal("first run")
			}
			timer := receive(t, clock.timers)
			if receive(t, clock.delay) != 5*time.Second {
				t.Fatal("default changed")
			}
			if count.Load() != 1 {
				t.Fatal("restart without clock advance")
			}
			timer.c <- time.Time{}
			receive(t, timer.stopped)
			if receive(t, calls) != 2 {
				t.Fatal("restart")
			}
			cancel()
			if !errors.Is(receive(t, done), context.Canceled) {
				t.Fatal("shutdown error")
			}
			if count.Load() != 2 || strings.Contains(logs.String(), "secret-token") || !strings.Contains(logs.String(), "reason=") {
				t.Fatal("restart or safe log contract", logs.String())
			}
		})
	}
}
func TestUnitCancellationStopsPendingRestartAndWaitsForCleanup(t *testing.T) {
	clock := newClock()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- worker.Run(ctx, worker.Options{Logger: logger(), Clock: clock, RestartDelay: time.Second}, worker.Job{Name: "one", Run: func(context.Context) error { return nil }})
	}()
	timer := receive(t, clock.timers)
	cancel()
	receive(t, timer.stopped)
	if !errors.Is(receive(t, done), context.Canceled) {
		t.Fatal("cancel")
	}
	ctx, cancel = context.WithCancel(t.Context())
	cancel()
	var ran atomic.Bool
	if !errors.Is(worker.Run(ctx, options(), worker.Job{Name: "never", Run: func(context.Context) error { ran.Store(true); return nil }}), context.Canceled) || ran.Load() {
		t.Fatal("cancelled supervisor ran job")
	}
}
func TestUnitEveryImmediateIterationCoalescesAndStopsTicker(t *testing.T) {
	clock := newClock()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	calls := make(chan struct{}, 3)
	var logs bytes.Buffer
	job, err := worker.Every("periodic", time.Minute, slog.New(slog.NewTextHandler(&logs, nil)), clock, func(context.Context) error { calls <- struct{}{}; return errors.New("private-payload") })
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- job.Run(ctx) }()
	ticker := receive(t, clock.tickers)
	receive(t, calls)
	if receive(t, clock.delay) != time.Minute {
		t.Fatal("interval")
	}
	ticker.c <- time.Time{}
	receive(t, calls)
	cancel()
	if receive(t, done) != nil {
		t.Fatal("schedule cancel")
	}
	receive(t, ticker.stopped)
	if strings.Contains(logs.String(), "private-payload") {
		t.Fatal("iteration error leaked")
	}
}
func TestUnitSystemClockSignalsCanBeStopped(t *testing.T) {
	clock := worker.SystemClock{}
	timer := clock.NewTimer(time.Hour)
	ticker := clock.NewTicker(time.Hour)
	if timer.C() == nil || ticker.C() == nil {
		t.Fatal("missing clock signal")
	}
	timer.Stop()
	ticker.Stop()
}

func TestUnitScheduledPanicStopsTickerBeforeSupervisorRestart(t *testing.T) {
	clock := newClock()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var calls atomic.Int32
	entered := make(chan struct{})
	job, err := worker.Every("periodic", time.Minute, logger(), clock, func(ctx context.Context) error {
		if calls.Add(1) == 1 {
			panic("private-payload")
		}
		close(entered)
		<-ctx.Done()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- worker.Run(ctx, worker.Options{Logger: logger(), Clock: clock, RestartDelay: time.Second}, job)
	}()
	oldTicker := receive(t, clock.tickers)
	receive(t, oldTicker.stopped)
	restart := receive(t, clock.timers)
	restart.c <- time.Time{}
	receive(t, restart.stopped)
	currentTicker := receive(t, clock.tickers)
	receive(t, entered)
	cancel()
	if !errors.Is(receive(t, done), context.Canceled) {
		t.Fatal("cancel")
	}
	receive(t, currentTicker.stopped)
}
func TestUnitEveryCancelledContextRetainsFirstIteration(t *testing.T) {
	clock := newClock()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	calls := 0
	job, err := worker.Every("initial", time.Hour, logger(), clock, func(context.Context) error { calls++; return errors.New("cancelled") })
	if err != nil {
		t.Fatal(err)
	}
	if err := job.Run(ctx); err != nil || calls != 1 {
		t.Fatal("first iteration semantics", err, calls)
	}
	receive(t, receive(t, clock.tickers).stopped)
}

func TestUnitEveryRetainsPendingTickDuringSlowFirstIteration(t *testing.T) {
	clock := newClock()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	release := make(chan struct{})
	first := make(chan *signal, 1)
	next := make(chan int32, 1)
	var calls, active atomic.Int32
	var overlapped atomic.Bool
	job, err := worker.Every("slow-first", time.Minute, logger(), clock, func(ctx context.Context) error {
		if active.Add(1) != 1 {
			overlapped.Store(true)
		}
		defer active.Add(-1)
		n := calls.Add(1)
		if n == 1 {
			// This observation happens inside the first callback: construction after
			// that callback would report nil instead of silently changing the schedule.
			select {
			case ticker := <-clock.tickers:
				first <- ticker
			default:
				first <- nil
			}
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		}
		next <- n
		cancel()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- job.Run(ctx) }()
	ticker := receive(t, first)
	if ticker == nil {
		t.Fatal("ticker must exist before the first callback")
	}
	ticker.c <- time.Time{} // A tick arrives while the first callback is still blocked.
	if calls.Load() != 1 || overlapped.Load() {
		t.Fatal("concurrent scheduled iteration")
	}
	close(release)
	// Do not advance the clock again: the tick held during the first call must suffice.
	if n := receive(t, next); n != 2 {
		t.Fatalf("pending tick started call %d", n)
	}
	if err := receive(t, done); err != nil {
		t.Fatal(err)
	}
	receive(t, ticker.stopped)
	if calls.Load() != 2 || active.Load() != 0 || overlapped.Load() {
		t.Fatal("schedule overlapped or retained a running callback")
	}
}

func TestUnitEveryCancellationWinsOverPendingTick(t *testing.T) {
	// A slow iteration lets a tick queue up; cancellation during that iteration must stop
	// the schedule even though both select cases are ready. Repeat to defeat select's
	// random choice instead of relying on timing.
	for range 200 {
		clock := newClock()
		ctx, cancel := context.WithCancel(t.Context())
		var calls atomic.Int32
		var logs bytes.Buffer
		job, err := worker.Every("slow", time.Minute, slog.New(slog.NewTextHandler(&logs, nil)), clock, func(ctx context.Context) error {
			if calls.Add(1) == 1 {
				ticker := receive(t, clock.tickers)
				ticker.c <- time.Time{}
				cancel()
			}
			return ctx.Err()
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := job.Run(ctx); err != nil {
			t.Fatal(err)
		}
		if n := calls.Load(); n != 1 {
			t.Fatalf("iteration ran after cancellation: %d calls", n)
		}
		if logs.Len() != 0 {
			t.Fatal("iteration error after cancellation was logged", logs.String())
		}
	}
}

func TestUnitEveryLogsBoundedIterationFailure(t *testing.T) {
	clock := newClock()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var logs bytes.Buffer
	calls := make(chan struct{}, 3)
	var n atomic.Int32
	job, err := worker.Every("partition-check", time.Minute, slog.New(slog.NewJSONHandler(&logs, nil)), clock, func(ctx context.Context) error {
		calls <- struct{}{}
		if n.Add(1) == 3 {
			// Entering the third call proves both earlier failures were logged; its
			// own failure follows cancellation and must not be logged.
			<-ctx.Done()
		}
		return errors.New("private-payload")
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- job.Run(ctx) }()
	ticker := receive(t, clock.tickers)
	receive(t, calls)
	for range 2 {
		ticker.c <- time.Time{}
		receive(t, calls)
	}
	cancel()
	if err := receive(t, done); err != nil {
		t.Fatal(err)
	}
	records := decodeLogs(t, &logs)
	if len(records) != 2 {
		t.Fatalf("want one record per failed iteration, got %d: %v", len(records), records)
	}
	for _, r := range records {
		if r["level"] != "ERROR" || r["msg"] != "job iteration failed" || r["job"] != "partition-check" || r["reason"] != "error" {
			t.Fatalf("iteration log contract: %v", r)
		}
	}
	if strings.Contains(logs.String(), "private-payload") {
		t.Fatal("iteration error leaked")
	}
}

func TestUnitRunLogsBoundedRestartReason(t *testing.T) {
	for mode, reason := range map[string]string{"error": "error", "nil": "returned", "panic": "panic"} {
		t.Run(mode, func(t *testing.T) {
			clock := newClock()
			var logs bytes.Buffer
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var calls atomic.Int32
			job := worker.Job{Name: "consumer", Run: func(ctx context.Context) error {
				if calls.Add(1) > 1 {
					<-ctx.Done()
					return errors.New("after-cancel")
				}
				switch mode {
				case "error":
					return errors.New("secret-token")
				case "panic":
					panic("secret-token")
				}
				return nil
			}}
			done := make(chan error, 1)
			go func() {
				done <- worker.Run(ctx, worker.Options{Logger: slog.New(slog.NewJSONHandler(&logs, nil)), Clock: clock, RestartDelay: 3 * time.Second}, job)
			}()
			timer := receive(t, clock.timers)
			timer.c <- time.Time{}
			receive(t, timer.stopped)
			cancel()
			if !errors.Is(receive(t, done), context.Canceled) {
				t.Fatal("shutdown")
			}
			records := decodeLogs(t, &logs)
			if len(records) != 1 {
				t.Fatalf("want exactly one restart record (none after cancellation), got %v", records)
			}
			r := records[0]
			if r["level"] != "ERROR" || r["msg"] != "job stopped, restarting" || r["job"] != "consumer" || r["reason"] != reason || r["delay"] != float64(3*time.Second) {
				t.Fatalf("restart log contract: %v", r)
			}
			if strings.Contains(logs.String(), "secret-token") {
				t.Fatal("payload leaked")
			}
		})
	}
}

func decodeLogs(t *testing.T, logs *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	dec := json.NewDecoder(logs)
	for dec.More() {
		var r map[string]any
		if err := dec.Decode(&r); err != nil {
			t.Fatal(err)
		}
		records = append(records, r)
	}
	return records
}
