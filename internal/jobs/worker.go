package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rollbar/rollbar-go"

	"github.com/sjperalta/fintera-api/pkg/logger"
)

// Job represents a background task
type Job func(ctx context.Context) error

// Worker manages background jobs and scheduled tasks
type Worker struct {
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	queue         chan Job
	asyncSem      chan struct{}
	maxConcurrent int
	stats         WorkerStats
	statsMu       sync.RWMutex
	mu            sync.RWMutex
	closed        bool
}

// WorkerStats holds statistics about the worker
type WorkerStats struct {
	ActiveJobs    int   `json:"active_jobs"`
	CompletedJobs int64 `json:"completed_jobs"`
	FailedJobs    int64 `json:"failed_jobs"`
	QueueLength   int   `json:"queue_length"`
	MaxConcurrent int   `json:"max_concurrent"`
}

// NewWorker creates a worker with N concurrent processors
func NewWorker(numWorkers int) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	// Allow 2x workers for async jobs
	asyncLimit := numWorkers * 2
	if asyncLimit < 10 {
		asyncLimit = 10
	}

	w := &Worker{
		ctx:           ctx,
		cancel:        cancel,
		queue:         make(chan Job, 100),
		asyncSem:      make(chan struct{}, asyncLimit),
		maxConcurrent: asyncLimit,
	}

	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		w.wg.Add(1)
		go w.process(i)
	}

	return w
}

// Enqueue adds a job to be processed by the worker pool
func (w *Worker) Enqueue(job Job) {
	w.mu.RLock()
	if w.closed {
		w.mu.RUnlock()
		return
	}
	select {
	case w.queue <- job:
		w.mu.RUnlock()
	default:
		w.mu.RUnlock()
		logger.Warn("[Worker] Queue full, running job synchronously")
		if err := w.execute(job); err != nil {
			logger.Error(fmt.Sprintf("[Worker] Job error: %v", err))
		}
	}
}

// EnqueueAsync runs a job in a new goroutine (fire-and-forget), bounded by semaphore
func (w *Worker) EnqueueAsync(job Job) {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.wg.Add(1)
	w.mu.Unlock()

	go func() {
		defer w.wg.Done()
		// Acquire semaphore to limit concurrency
		w.asyncSem <- struct{}{}
		defer func() { <-w.asyncSem }()

		w.trackJobStart()
		defer w.trackJobEnd()

		if err := w.execute(job); err != nil {
			logger.Error(fmt.Sprintf("[Worker] Async job error: %v", err))
			w.trackJobFailure()
		}
	}()
}

// execute runs a job and converts panics into errors, reporting them to
// Rollbar. Background goroutines are not covered by Gin's recovery middleware,
// so without this a panicking job would crash the whole process.
func (w *Worker) execute(job Job) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = panicError(r)
			reportToRollbar(err)
		}
	}()
	return job(w.ctx)
}

// process handles jobs from the queue
func (w *Worker) process(workerID int) {
	defer w.wg.Done()
	for {
		select {
		case <-w.ctx.Done():
			return
		case job, ok := <-w.queue:
			if !ok {
				return
			}
			w.trackJobStart()
			start := time.Now()
			if err := w.execute(job); err != nil {
				logger.Error(fmt.Sprintf("[Worker %d] Job error: %v", workerID, err))
				w.trackJobFailure()
			} else {
				logger.Info(fmt.Sprintf("[Worker %d] Job completed in %v", workerID, time.Since(start)))
			}
			w.trackJobEnd()
		}
	}
}

// ScheduleEvery runs a job at fixed intervals. The first run happens after the interval (not at startup).
func (w *Worker) ScheduleEvery(interval time.Duration, job Job) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-w.ctx.Done():
				return
			case <-ticker.C:
				w.runScheduledJob(job)
			}
		}
	}()
}

// ScheduleEveryImmediate runs a job once at startup, then at fixed intervals. Use this when the process
// may restart (e.g. Railway deploys) so jobs run soon after start instead of waiting for the first interval.
func (w *Worker) ScheduleEveryImmediate(interval time.Duration, job Job) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.runScheduledJob(job)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-w.ctx.Done():
				return
			case <-ticker.C:
				w.runScheduledJob(job)
			}
		}
	}()
}

func (w *Worker) runScheduledJob(job Job) {
	w.trackJobStart()
	start := time.Now()
	if err := w.execute(job); err != nil {
		logger.Error(fmt.Sprintf("[Scheduler] Job error: %v", err))
		w.trackJobFailure()
	} else {
		logger.Info(fmt.Sprintf("[Scheduler] Job completed in %v", time.Since(start)))
	}
	w.trackJobEnd()
}

// ScheduleAt runs a job once at a specific time
func (w *Worker) ScheduleAt(at time.Time, job Job) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		timer := time.NewTimer(time.Until(at))
		defer timer.Stop()

		select {
		case <-w.ctx.Done():
			return
		case <-timer.C:
			w.trackJobStart()
			if err := w.execute(job); err != nil {
				logger.Error(fmt.Sprintf("[Scheduler] Scheduled job error: %v", err))
				w.trackJobFailure()
			}
			w.trackJobEnd()
		}
	}()
}

// Shutdown gracefully stops all workers
func (w *Worker) Shutdown() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	w.cancel()
	close(w.queue)
	w.mu.Unlock()
	w.wg.Wait()
}

// Context returns the worker's context for checking cancellation
func (w *Worker) Context() context.Context {
	return w.ctx
}

// GetStats returns the current worker statistics
func (w *Worker) GetStats() WorkerStats {
	w.statsMu.RLock()
	defer w.statsMu.RUnlock()
	stats := w.stats
	stats.QueueLength = len(w.queue)
	stats.MaxConcurrent = w.maxConcurrent
	return stats
}

// panicError normalizes a recover() value into an error. Plain string panics
// (panic("boom")) are the common case, and the SDK only captures a stack trace
// when the reported value implements error.
func panicError(r interface{}) error {
	if err, ok := r.(error); ok {
		return err
	}
	return fmt.Errorf("panic: %v", r)
}

// reportToRollbar sends background job panic reports to Rollbar. It is a no-op
// when no token is configured, so local and test runs never make network calls.
func reportToRollbar(err error) {
	if err == nil || rollbar.Token() == "" {
		return
	}
	rollbar.ErrorWithLevel(rollbar.ERR, err)
}

func (w *Worker) trackJobStart() {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()
	w.stats.ActiveJobs++
}

func (w *Worker) trackJobEnd() {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()
	w.stats.ActiveJobs--
	w.stats.CompletedJobs++
}

func (w *Worker) trackJobFailure() {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()
	w.stats.FailedJobs++
	// CompletedJobs is incremented in trackJobEnd, which is always called.
	// So we don't decrement CompletedJobs here, total jobs = completed + (failed included in completed count? No, typically separate or failed is subset).
	// Let's enable CompletedJobs to be total finished. FailedJobs is a subset or separate counter?
	// Common pattern: Completed = Success, Failed = Failure.
	// But here trackJobEnd is always called.
	// Let's adjust: trackJobEnd increments Completed. If it failed, we also increment Failed.
	// So CompletedJobs effectively means "Finished Jobs" (success or fail).
	// If we want Success count, we can derive it: Completed - Failed.
}
