package utils

import (
	"context"
	"fmt"
	"sync"
	"time"
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

// Submit 向任务队列提交一个任务（阻塞等待）
func (wp *WorkerPool) Submit(task func()) {
	select {
	case wp.taskQueue <- task:
	case <-wp.ctx.Done():
	}
}

// TrySubmit 尝试提交任务，如果队列满则立即返回false
func (wp *WorkerPool) TrySubmit(task func()) bool {
	select {
	case wp.taskQueue <- task:
		return true
	case <-wp.ctx.Done():
		return false
	default:
		return false // 队列满了
	}
}

// SubmitWithTimeout 带超时的任务提交
func (wp *WorkerPool) SubmitWithTimeout(task func(), timeout time.Duration) error {
	select {
	case wp.taskQueue <- task:
		return nil
	case <-wp.ctx.Done():
		return fmt.Errorf("worker pool已关闭")
	case <-time.After(timeout):
		return fmt.Errorf("提交任务超时")
	}
}

// Stop 停止所有 worker，并等待其退出
func (wp *WorkerPool) Stop() {
	wp.cancel()
	wp.wg.Wait()
}

// QueueSize 返回当前队列中的任务数量
func (wp *WorkerPool) QueueSize() int {
	return len(wp.taskQueue)
}

// QueueCapacity 返回队列的容量
func (wp *WorkerPool) QueueCapacity() int {
	return cap(wp.taskQueue)
}

// IsFull 检查队列是否已满
func (wp *WorkerPool) IsFull() bool {
	return len(wp.taskQueue) == cap(wp.taskQueue)
}
