package worker

import "time"

// Timer is a one-shot clock signal. Stop releases its resources, including after firing.
type Timer interface {
	C() <-chan time.Time
	Stop()
}

// Ticker is a periodic clock signal. Stop releases its resources.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

// Clock supplies concurrency-safe timers and tickers; implementations must accept
// positive durations and return non-nil signals. SystemClock uses the standard library.
type Clock interface {
	NewTimer(time.Duration) Timer
	NewTicker(time.Duration) Ticker
}

// SystemClock uses standard-library timers and coalescing tickers. Its zero value is
// ready for use and safe for concurrent use; it has no mutable process-wide settings.
type SystemClock struct{}

// NewTimer creates a one-shot timer.
func (SystemClock) NewTimer(d time.Duration) Timer { return systemTimer{time.NewTimer(d)} }

// NewTicker creates a periodic ticker.
func (SystemClock) NewTicker(d time.Duration) Ticker { return systemTicker{time.NewTicker(d)} }

type systemTimer struct{ timer *time.Timer }

func (t systemTimer) C() <-chan time.Time { return t.timer.C }
func (t systemTimer) Stop()               { t.timer.Stop() }

type systemTicker struct{ ticker *time.Ticker }

func (t systemTicker) C() <-chan time.Time { return t.ticker.C }
func (t systemTicker) Stop()               { t.ticker.Stop() }
