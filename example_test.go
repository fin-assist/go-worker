package worker_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	worker "github.com/fin-assist/go-worker"
)

func ExampleRun() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := worker.Run(ctx, worker.Options{Logger: slog.New(slog.DiscardHandler), Clock: worker.SystemClock{}}, worker.Job{Name: "consumer", Run: func(ctx context.Context) error {
		// The service owns shutdown; the job finishes its cleanup before returning.
		cancel()
		return ctx.Err()
	}})
	fmt.Println(errors.Is(err, context.Canceled))
	// Output: true
}

func ExampleEvery() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	job, err := worker.Every("immediate", time.Minute, slog.New(slog.DiscardHandler), worker.SystemClock{}, func(context.Context) error {
		calls++
		cancel()
		return nil
	})
	if err != nil {
		panic(err)
	}
	if err := job.Run(ctx); err != nil {
		panic(err)
	}
	fmt.Println(calls)
	// Output: 1
}
