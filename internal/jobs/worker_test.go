package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExecuteRecoversStringPanic(t *testing.T) {
	w := NewWorker(1)
	defer w.Shutdown()

	err := w.execute(func(context.Context) error {
		panic("boom")
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "panic: boom")
}

func TestExecutePreservesErrorPanic(t *testing.T) {
	w := NewWorker(1)
	defer w.Shutdown()

	sentinel := errors.New("kaboom")
	err := w.execute(func(context.Context) error {
		panic(sentinel)
	})

	require.ErrorIs(t, err, sentinel)
}

func TestExecuteReturnsJobError(t *testing.T) {
	w := NewWorker(1)
	defer w.Shutdown()

	sentinel := errors.New("job failed")
	err := w.execute(func(context.Context) error {
		return sentinel
	})

	require.ErrorIs(t, err, sentinel)
}

func TestEnqueueAsyncSurvivesPanicAndTracksFailure(t *testing.T) {
	w := NewWorker(1)
	defer w.Shutdown()

	started := make(chan struct{})
	w.EnqueueAsync(func(context.Context) error {
		close(started)
		panic("async boom")
	})

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("async job did not start")
	}

	require.Eventually(t, func() bool {
		return w.GetStats().FailedJobs == 1
	}, 2*time.Second, 10*time.Millisecond)
}

func TestScheduledJobSurvivesPanic(t *testing.T) {
	w := NewWorker(1)
	defer w.Shutdown()

	started := make(chan struct{})
	w.ScheduleEveryImmediate(time.Hour, func(context.Context) error {
		close(started)
		panic("scheduled boom")
	})

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduled job did not run")
	}

	require.Eventually(t, func() bool {
		return w.GetStats().FailedJobs == 1
	}, 2*time.Second, 10*time.Millisecond)
}
