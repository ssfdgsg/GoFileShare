package utils

import "context"

type WorkerPool struct {
	workerCount int
	taskQueue   chan func()
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

func NewWorkerPool(workerCount int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		workerCount: workerCount,
		taskQueue:   make(chan func(), 100),
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workerCount; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
}

func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	for {
		select {
		case task := <-wp.taskQueue:
			task()
		case <-wp.ctx.Done():
			return
		}
	}
}

func (wp *WorkerPool) Submit(task func()) {
	select {
	case wp.taskQueue <- task:
	case <-wp.ctx.Done():
	}
}

func (wp *WorkerPool) Stop() {
	wp.cancel()
	wp.wg.Wait()
}
