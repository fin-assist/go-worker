// Package worker supervises cooperative service-owned background jobs. Run waits
// for cancellation cleanup, restarts failed loops and contains panics. Every runs
// immediate-first periodic work. Callers inject their logger, clock and restart
// policy; jobs remain responsible for distributed locking and idempotent effects.
package worker
