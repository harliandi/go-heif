package converter

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

var (
	// ErrPoolBusy is returned when the worker pool is at capacity
	ErrPoolBusy = errors.New("worker pool is busy, please retry later")
)

// Job represents a conversion job
type Job struct {
	Data   []byte
	Scale  float64
	Quality int
	Result chan<- Result
}

// Result represents the outcome of a conversion job
type Result struct {
	Data []byte
	Err  error
}

// WorkerPool manages a pool of worker goroutines for conversion jobs
type WorkerPool struct {
	jobs    chan Job
	workers int
	wg      sync.WaitGroup
	once    sync.Once
}

// NewWorkerPool creates a new worker pool with the specified number of workers
func NewWorkerPool(workers int) *WorkerPool {
	return &WorkerPool{
		jobs:    make(chan Job, workers*2), // Buffered channel
		workers: workers,
	}
}

// Start starts the worker pool goroutines
func (p *WorkerPool) Start() {
	p.once.Do(func() {
		log.Printf("Starting worker pool with %d workers", p.workers)
		for i := 0; i < p.workers; i++ {
			p.wg.Add(1)
			go p.worker(i)
		}
	})
}

// worker processes jobs from the job channel
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()
	for job := range p.jobs {
		// Process the job
		var result Result
		if job.Quality > 0 && job.Scale > 0 && job.Scale < 1.0 {
			result.Data, result.Err = defaultPool.ConvertBytesFastWithQuality(job.Data, job.Scale, job.Quality)
		} else if job.Scale > 0 && job.Scale < 1.0 {
			result.Data, result.Err = defaultPool.ConvertBytesFast(job.Data, job.Scale)
		} else if job.Quality > 0 {
			result.Data, result.Err = defaultPool.ConvertBytesWithQuality(job.Data, job.Quality)
		} else {
			result.Data, result.Err = defaultPool.ConvertBytes(job.Data)
		}

		// Send result (non-blocking in case receiver is gone)
		select {
		case job.Result <- result:
		default:
			log.Printf("Worker %d: result channel full or closed", id)
		}
	}
}

// Submit submits a job to the worker pool with context cancellation support
// Returns ErrPoolBusy if the worker pool queue is full
func (p *WorkerPool) Submit(ctx context.Context, data []byte, scale float64, quality int) ([]byte, error) {
	// Start the pool if not already started
	p.Start()

	resultChan := make(chan Result, 1)
	job := Job{
		Data:   data,
		Scale:  scale,
		Quality: quality,
		Result: resultChan,
	}

	// Try to submit job with a timeout to avoid blocking indefinitely
	// If the queue is full, return ErrPoolBusy immediately
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case p.jobs <- job:
		// Job submitted, wait for result
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case result := <-resultChan:
			return result.Data, result.Err
		}
	default:
		// Queue is full, return busy error
		return nil, ErrPoolBusy
	}
}

// SubmitWithRetry submits a job to the worker pool with retry on busy
func (p *WorkerPool) SubmitWithRetry(ctx context.Context, data []byte, scale float64, quality int, maxRetries int) ([]byte, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		result, err := p.Submit(ctx, data, scale, quality)
		if err == nil {
			return result, nil
		}
		if !errors.Is(err, ErrPoolBusy) {
			return nil, err
		}
		lastErr = err

		// Wait a bit before retry (exponential backoff)
		waitTime := time.Duration(i+1) * 10 * time.Millisecond
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(waitTime):
		}
	}
	return nil, lastErr
}

// Stop gracefully shuts down the worker pool
func (p *WorkerPool) Stop() {
	close(p.jobs)
	p.wg.Wait()
	log.Printf("Worker pool stopped")
}

// Stats returns current pool statistics
func (p *WorkerPool) Stats() (active, queued int) {
	return len(p.jobs), cap(p.jobs) - len(p.jobs)
}

// Default global worker pool
var (
	defaultPool     *Converter
	globalWorkerPool *WorkerPool
	poolInitOnce    sync.Once
)

// InitGlobalWorkerPool initializes the global worker pool
func InitGlobalWorkerPool(workers int, targetSizeKB int) {
	poolInitOnce.Do(func() {
		defaultPool = New(targetSizeKB)
		globalWorkerPool = NewWorkerPool(workers)
		globalWorkerPool.Start()
	})
}

// SubmitToGlobalPool submits a job to the global worker pool
func SubmitToGlobalPool(ctx context.Context, data []byte, scale float64, quality int) ([]byte, error) {
	if globalWorkerPool == nil {
		// Fallback to direct conversion if pool not initialized
		if quality > 0 && scale > 0 && scale < 1.0 {
			return defaultPool.ConvertBytesFastWithQuality(data, scale, quality)
		} else if scale > 0 && scale < 1.0 {
			return defaultPool.ConvertBytesFast(data, scale)
		} else if quality > 0 {
			return defaultPool.ConvertBytesWithQuality(data, quality)
		}
		return defaultPool.ConvertBytes(data)
	}
	return globalWorkerPool.Submit(ctx, data, scale, quality)
}
