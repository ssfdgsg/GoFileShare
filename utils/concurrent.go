package utils

import (
	"context"
	"sync"
)

// WorkerPool 表示一个简单的协程池，用于并发执行任务
type WorkerPool struct {
	WorkerCount int
	taskQueue   chan func()
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewWorkerPool 创建一个新的 WorkerPool 实例
func NewWorkerPool(workerCount int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		WorkerCount: workerCount,
		taskQueue:   make(chan func(), 100),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start 启动所有 worker 协程，开始处理任务
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.WorkerCount; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
}

// worker 是实际执行任务的协程函数
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

// Submit 向任务队列提交一个任务
func (wp *WorkerPool) Submit(task func()) {
	select {
	case wp.taskQueue <- task:
	case <-wp.ctx.Done():
	}
}

// Stop 停止所有 worker，并等待其退出
func (wp *WorkerPool) Stop() {
	wp.cancel()
	wp.wg.Wait()
}
