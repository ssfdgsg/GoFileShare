package services

import (
    "fmt"
    "sync"
    "time"
    
    "github.com/fatih/color"
    "GoFileShare/utils"
)

type DownloadService struct {
	workerPool *utils.WorkerPool
	activeJobs map[string]*DownloadTask
	jobsMutex     sync.RWMutex
    completedJobs int
    totalJobs     int
    wg            sync.WaitGroup
}

func NewDownloadService(workerCount int) *DownloadService {
	return &DownloadService{
		workerPool: utils.NewWorkerPool(workerCount),
		activeJobs: make(map[string]*DownloadTask),
		completedJobs: 0,
		totalJobs: 0,
	}
}

func (DownloadManager *DownloadService) Start() {
	DownloadManager.workerPool.Start()
	color.Green("Download service started with %d workers", DownloadManager.workerPool.workerCount)
}

func (DownloadManager *DownloadService) SubmitDownloadTask() {
	task.OnComplete = DownloadManager.onTaskComplete
	task.OnError = DownloadManager.onTaskError

	DownloadManager.jobsMutex.Lock()
	DownloadManager.activeJobs[task.DownloadUrl] = task
	DownloadManager.totalJobs++
	DownloadManager.jobsMutex.Unlock()

	DownloadManager.wg.Add(1)
	DownloadManager.workerPool.Submit(func() {
		defer DownloadManager.wg.Done()
		task.Execute()
	})
}

func (DownloadManager *DownloadService) onTaskComplete(task *DownloadTask) {
	DownloadManager.jobsMutex.Lock()
	delete(DownloadManager.activeJobs, task.DownloadUrl)
	DownloadManager.completedJobs++
	DownloadManager.completedJobs++
	DownloadManager.jobsMutex.Unlock()
	color.Green("Task %d completed successfully: %s", task.OrderId, task.DownloadUrl)
	DownloadManager.printProgress()
}

func (DownloadManager *DownloadService) onTaskError(task *DownloadTask, err error) {
	DownloadManager.jobsMutex.Lock()
	delete(DownloadManager.activeJobs, task.DownloadUrl)
	DownloadManager.jobsMutex.Unlock()
	color.Red("Task %d failed: %s", task.OrderId, err.Error())
}

func (DownloadManager *DownloadService) printProgress() {
	DownloadManager.jobsMutex.RLock()
	activeCount := len(DownloadManager.activeJobs)
	completedCount := DownloadManager.completedJobs
	totalCount := DownloadManager.totalJobs
	DownloadManager.jobsMutex.RUnlock()
	if totalCount > 0 {
		progress := float64(completedCount) / float64(totalCount) * 100
		color.Blue("Download progress: %.2f%% (Active: %d, Completed: %d, Total: %d)",
			progress, activeCount, completedCount, totalCount)
	}	
}

func (DownloadManager *DownloadService) WaitForAll() {
	DownloadManager.wg.Wait()
	color.Green("All download tasks completed. Total: %d, Completed: %d", DownloadManager.totalJobs, DownloadManager.completedJobs)
}

func (DownloadManager *DownloadService) Stop() {
	DownloadManager.workerPool.Stop()
	color.Yellow("Download service stopped")
}
