package workerpool

import (
	"cloudflare-speedtest/internal/colomanager"
	"cloudflare-speedtest/internal/tester"
	"cloudflare-speedtest/pkg/models"
	"context"
	"fmt"
	"sync"
	"time"
)

// TestJob represents a single IP test job
type TestJob struct {
	IP           string
	UseTLS       bool
	Timeout      int
	DownloadTime float64
	TestURL      string
	Domain       string
	FilePath     string
	Phase        string // "datacenter" or "speed"
}

// TestResult represents the result of a test phase
type TestResult struct {
	IP         string
	DataCenter string
	Latency    float64
	Phase      string
	Error      error
}

// WorkerPool manages concurrent IP testing
type WorkerPool struct {
	workerCount int
	jobQueue    chan TestJob
	resultQueue chan *models.SpeedTestResult
	workers     []*Worker
	quit        chan struct{}
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.RWMutex
	running     bool
	coloManager *colomanager.DataCenterManager
}

// Worker represents a single worker goroutine
type Worker struct {
	id          int
	jobQueue    <-chan TestJob
	resultQueue chan<- *models.SpeedTestResult
	speedTester *tester.SpeedTester
	quit        chan struct{}
	wg          *sync.WaitGroup
	coloManager *colomanager.DataCenterManager
}

// New creates a new worker pool
func New(workerCount int, coloManager *colomanager.DataCenterManager) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		workerCount: workerCount,
		jobQueue:    make(chan TestJob, workerCount*2), // Buffer for better performance
		resultQueue: make(chan *models.SpeedTestResult, workerCount*2),
		workers:     make([]*Worker, 0, workerCount),
		quit:        make(chan struct{}),
		ctx:         ctx,
		cancel:      cancel,
		running:     false,
		coloManager: coloManager,
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start() error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.running {
		return fmt.Errorf("worker pool is already running")
	}

	fmt.Printf("Starting worker pool with %d workers\n", wp.workerCount)

	// Create and start workers
	for i := 0; i < wp.workerCount; i++ {
		worker := &Worker{
			id:          i + 1,
			jobQueue:    wp.jobQueue,
			resultQueue: wp.resultQueue,
			speedTester: tester.New(30), // 30 second timeout for workers
			quit:        make(chan struct{}),
			wg:          &wp.wg,
			coloManager: wp.coloManager,
		}

		wp.workers = append(wp.workers, worker)
		wp.wg.Add(1)
		go worker.Start(wp.ctx)
	}

	wp.running = true
	fmt.Printf("Worker pool started successfully with %d workers\n", len(wp.workers))
	return nil
}

// Stop gracefully stops the worker pool
func (wp *WorkerPool) Stop() error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if !wp.running {
		return fmt.Errorf("worker pool is not running")
	}

	fmt.Println("Stopping worker pool...")

	// Cancel context to signal all workers to stop
	wp.cancel()

	// Close job queue to prevent new jobs (only if not already closed)
	select {
	case <-wp.jobQueue:
		// Channel is already closed or empty
	default:
		close(wp.jobQueue)
	}

	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("All workers stopped gracefully")
	case <-time.After(10 * time.Second):
		fmt.Println("Timeout waiting for workers to stop, forcing shutdown")
	}

	// Close result queue (only if not already closed)
	select {
	case <-wp.resultQueue:
		// Channel is already closed or empty
	default:
		close(wp.resultQueue)
	}

	wp.running = false
	wp.workers = wp.workers[:0] // Clear workers slice

	// Create new context and channels for potential restart
	wp.ctx, wp.cancel = context.WithCancel(context.Background())
	wp.jobQueue = make(chan TestJob, wp.workerCount*2)
	wp.resultQueue = make(chan *models.SpeedTestResult, wp.workerCount*2)

	fmt.Println("Worker pool stopped")
	return nil
}

// AddJob adds a job to the worker pool
func (wp *WorkerPool) AddJob(job TestJob) error {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	if !wp.running {
		return fmt.Errorf("worker pool is not running")
	}

	select {
	case wp.jobQueue <- job:
		return nil
	case <-wp.ctx.Done():
		return fmt.Errorf("worker pool is shutting down")
	default:
		return fmt.Errorf("job queue is full")
	}
}

// GetResults returns the result channel for reading test results
func (wp *WorkerPool) GetResults() <-chan *models.SpeedTestResult {
	return wp.resultQueue
}

// IsRunning returns whether the worker pool is currently running
func (wp *WorkerPool) IsRunning() bool {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return wp.running
}

// GetWorkerCount returns the number of workers in the pool
func (wp *WorkerPool) GetWorkerCount() int {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return wp.workerCount
}

// GetActiveWorkers returns the number of currently active workers
func (wp *WorkerPool) GetActiveWorkers() int {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return len(wp.workers)
}

// Start starts the worker
func (w *Worker) Start(ctx context.Context) {
	defer w.wg.Done()

	fmt.Printf("Worker %d started\n", w.id)

	for {
		select {
		case job, ok := <-w.jobQueue:
			if !ok {
				fmt.Printf("Worker %d: job queue closed, stopping\n", w.id)
				return
			}

			fmt.Printf("Worker %d: processing job for IP %s\n", w.id, job.IP)
			result := w.processJob(job)

			select {
			case w.resultQueue <- result:
				fmt.Printf("Worker %d: result sent for IP %s\n", w.id, job.IP)
			case <-ctx.Done():
				fmt.Printf("Worker %d: context cancelled while sending result\n", w.id)
				return
			}

		case <-ctx.Done():
			fmt.Printf("Worker %d: context cancelled, stopping\n", w.id)
			return
		}
	}
}

// processJob processes a single test job
func (w *Worker) processJob(job TestJob) *models.SpeedTestResult {
	// Configure the speed tester for this job
	w.speedTester.SetConfig(job.Domain, job.FilePath, job.DownloadTime)

	// Perform the speed test
	result := w.speedTester.TestSpeed(job.IP, job.UseTLS, job.Timeout, job.DownloadTime)

	// Apply data center filtering if available
	if w.coloManager != nil && result.DataCenter != "" {
		if !w.coloManager.FilterByDataCenter(result.DataCenter) {
			fmt.Printf("Worker %d: IP %s filtered out (datacenter: %s not in selected list)\n",
				w.id, job.IP, result.DataCenter)
			result.Status = "已跳过"
			result.Speed = "-"
		} else {
			// Convert data center code to friendly name
			friendlyName := w.coloManager.GetFriendlyName(result.DataCenter)
			result.DataCenter = friendlyName
		}
	}

	return result
}

// WorkerPoolStats represents statistics about the worker pool
type WorkerPoolStats struct {
	WorkerCount     int  `json:"worker_count"`
	ActiveWorkers   int  `json:"active_workers"`
	Running         bool `json:"running"`
	JobQueueSize    int  `json:"job_queue_size"`
	ResultQueueSize int  `json:"result_queue_size"`
}

// GetStats returns current statistics about the worker pool
func (wp *WorkerPool) GetStats() WorkerPoolStats {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	return WorkerPoolStats{
		WorkerCount:     wp.workerCount,
		ActiveWorkers:   len(wp.workers),
		Running:         wp.running,
		JobQueueSize:    len(wp.jobQueue),
		ResultQueueSize: len(wp.resultQueue),
	}
}
