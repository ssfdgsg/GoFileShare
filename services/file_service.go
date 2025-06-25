package services

import (
	"GoFileShare/models"
	"GoFileShare/utils"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fatih/color"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TransferService 统一的文件传输服务
type TransferService struct {
	config     models.TransferConfig
	workerPool *utils.WorkerPool
	activeJobs map[string]*FileTask
	jobsMutex  sync.RWMutex
	stopCh     chan struct{}
}

// FileTask 文件传输任务
type FileTask struct {
	ID          string
	URL         string  // 下载URL或上传目标
	FilePath    string  // 本地文件路径
	FileName    string  // 文件名
	FileSize    int64   // 文件大小
	ChunkSize   int64   // 分块大小
	ChunkStatus []int64 // 使用数字记录已经完成了多少块，下次开始的时候指针直接偏移
	Progress    float64
	Completed   bool
	TaskType    string // "download" 或 "upload"
	OnProgress  func(float64)
	OnComplete  func(*FileTask)
	OnError     func(*FileTask, error)
	cancel      chan struct{}
}

// NewTransferService 创建传输服务
func NewTransferService(config models.TransferConfig) *TransferService {
	// 确保元数据目录存在
	err := os.MkdirAll(config.MetaDir, 0755)
	if err != nil {
		color.Red("Error creating meta directory %s: %v", config.MetaDir, err)
		return nil
	}

	return &TransferService{
		config:     config,
		workerPool: utils.NewWorkerPool(config.WorkerCount),
		activeJobs: make(map[string]*FileTask),
		stopCh:     make(chan struct{}),
	}
}

// Start 启动传输服务
func (s *TransferService) Start() {
	s.workerPool.Start()

	// 启动状态保存协程
	go s.periodicStatusSave()
}

// Stop 停止传输服务
func (s *TransferService) Stop() {
	close(s.stopCh)
	s.workerPool.Stop()

	// 保存所有任务状态
	s.jobsMutex.Lock()
	defer s.jobsMutex.Unlock()

	for _, task := range s.activeJobs {
		err := s.saveTaskStatus(task)
		if err != nil {
			color.Red("Error saving task status: %v", err)
			return
		}
	}
}

// GetTaskStatus 获取任务状态
func (s *TransferService) GetTaskStatus(taskID string) (*FileTask, bool) {
	s.jobsMutex.RLock()
	defer s.jobsMutex.RUnlock()

	task, exists := s.activeJobs[taskID]
	return task, exists
}

// 保存任务状态
func (s *TransferService) saveTaskStatus(task *FileTask) error {
	metaFile := filepath.Join(s.config.MetaDir, task.ID+".json")

	metaData := models.TaskMetadata{
		ID:           task.ID,
		CreatedTime:  time.Now(),
		LastModified: time.Now(),
		FilePath:     task.FilePath,
		FileName:     task.FileName,
		TotalSize:    task.FileSize,
		ChunkSize:    task.ChunkSize,
		ChunkStatus:  task.ChunkStatus,
		Progress:     task.Progress,
		Completed:    task.Completed,
		TaskType:     task.TaskType,
		URL:          task.URL,
	}

	jsonData, err := json.MarshalIndent(metaData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metaFile, jsonData, 0644)
}

// 加载任务状态
func (s *TransferService) loadTaskState(task *FileTask) error {
	metaFile := filepath.Join(s.config.MetaDir, task.ID+".json")
	data, err := os.ReadFile(metaFile)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在不是错误，初始化一个新任务
			chunkCount := int(math.Ceil(float64(task.FileSize) / float64(task.ChunkSize)))
			task.ChunkStatus = make([]int64, chunkCount)
			return nil
		}
		return err
	}

	var meta models.TaskMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		color.Red("Error unmarshalling task metadata for %s: %v", task.ID, err)
		return err
	}

	task.ChunkStatus = meta.ChunkStatus
	task.Progress = meta.Progress
	task.Completed = meta.Completed

	return nil
}

// 定期保存状态
func (s *TransferService) periodicStatusSave() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.jobsMutex.RLock()
			for _, task := range s.activeJobs {
				err := s.saveTaskStatus(task)
				if err != nil {
					color.Red("Error saving task status for %s: %v", task.ID, err)
					return
				}
			}
			s.jobsMutex.RUnlock()
		case <-s.stopCh:
			return
		}
	}
}

// AddDownloadTask 添加下载任务
func (s *TransferService) AddDownloadTask(url, filePath string, onProgress func(float64), onComplete func(*FileTask), onError func(*FileTask, error)) string {
	// 创建完整的文件路径
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		if onError != nil {
			color.Red("Error creating directory for %s: %s", filePath, err)
		}
		return ""
	}

	// 获取文件大小
	resp, err := http.Head(url)
	if err != nil {
		if onError != nil {
			color.Red("Error HEADing %s: %s", filePath, err)
		}
		return ""
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			if onError != nil {
				color.Red("Error closing response body: %v", err)
			}
		}
	}(resp.Body)

	fileSize := resp.ContentLength

	task := &FileTask{
		ID:         fmt.Sprintf("dl_%d", time.Now().UnixNano()),
		URL:        url,
		FilePath:   filePath,
		FileName:   filepath.Base(filePath),
		FileSize:   fileSize,
		ChunkSize:  s.config.ChunkSize,
		TaskType:   "download",
		OnProgress: onProgress,
		OnComplete: onComplete,
		OnError:    onError,
		cancel:     make(chan struct{}),
	}

	// 初始化或加载状态
	err = s.loadTaskState(task)
	if err != nil {
		color.Red("Error saving task status for %s: %v", task.ID, err)
		return ""
	}

	// 添加到活动任务列表
	s.jobsMutex.Lock()
	s.activeJobs[task.ID] = task
	s.jobsMutex.Unlock()

	// 提交下载任务
	s.workerPool.Submit(func() {
		if err := s.processDownload(task); err != nil {
			if task.OnError != nil {
				task.OnError(task, err)
			}
		} else if task.OnComplete != nil {
			task.OnComplete(task)
		}

		// 任务完成，从活动列表中移除
		s.jobsMutex.Lock()
		delete(s.activeJobs, task.ID)
		s.jobsMutex.Unlock()
	})

	return task.ID
}

// AddUploadTask 添加上传任务
func (s *TransferService) AddUploadTask(filePath, destination string, onProgress func(float64), onComplete func(*FileTask), onError func(*FileTask, error)) (string, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if onError != nil {
			onError(nil, err)
		}
		return "", err
	}

	task := &FileTask{
		ID:         fmt.Sprintf("up_%d", time.Now().UnixNano()),
		URL:        destination,
		FilePath:   filePath,
		FileName:   filepath.Base(filePath),
		FileSize:   fileInfo.Size(),
		ChunkSize:  s.config.ChunkSize,
		TaskType:   "upload",
		OnProgress: onProgress,
		OnComplete: onComplete,
		OnError:    onError,
		cancel:     make(chan struct{}),
	}

	// 初始化或加载状态
	err = s.loadTaskState(task)
	if err != nil {
		color.Red("Error loading task state for %s: %v", task.ID, err)
		return "", err
	}

	// 添加到活动任务列表
	s.jobsMutex.Lock()
	s.activeJobs[task.ID] = task
	s.jobsMutex.Unlock()

	// 提交上传任务
	s.workerPool.Submit(func() {
		if err := s.processUpload(task); err != nil {
			if task.OnError != nil {
				task.OnError(task, err)
			}
		} else if task.OnComplete != nil {
			task.OnComplete(task)
		}

		// 任务完成，从活动列表中移除
		s.jobsMutex.Lock()
		delete(s.activeJobs, task.ID)
		s.jobsMutex.Unlock()
	})

	return task.ID, nil
}

// 处理下载任务
func (s *TransferService) processDownload(task *FileTask) error {
	// 确保文件夹存在
	if err := os.MkdirAll(filepath.Dir(task.FilePath), 0755); err != nil {
		return err
	}

	// 创建临时文件
	tempFile := task.FilePath + ".download"
	file, err := os.OpenFile(tempFile, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			color.Red("Error closing file %s: %v", tempFile, err)
		}
	}(file)

	// 计算块数量
	chunkCount := int(math.Ceil(float64(task.FileSize) / float64(task.ChunkSize)))

	// 确保文件大小正确
	if err := file.Truncate(task.FileSize); err != nil {
		return err
	}

	// 初始化或加载状态
	threadStatus := make(map[int]int) // 线程ID -> 最后处理的块索引
	metaFile := filepath.Join(s.config.MetaDir, task.ID+".json")
	if data, err := os.ReadFile(metaFile); err == nil {
		var meta models.TaskMetadata
		if json.Unmarshal(data, &meta) == nil && meta.ThreadStatus != nil {
			threadStatus = meta.ThreadStatus
		}
	}

	// 初始化ChunkStatus
	for index := 0; index < chunkCount; index++ {
		task.ChunkStatus[index] = int64(threadStatus[index])
	}

	// 计算已完成块数
	completedChunks := 0
	for i := 0; i < len(task.ChunkStatus); i++ {
		if task.ChunkStatus[i] == '1' {
			completedChunks++
		}
	}

	// 同步机制
	var wg sync.WaitGroup
	var progressMutex sync.Mutex
	errorCh := make(chan error, 1)
	doneCh := make(chan struct{})

	// 分配块给线程
	workerCount := s.config.WorkerCount
	chunksPerWorker := chunkCount / workerCount
	if chunksPerWorker == 0 {
		chunksPerWorker = 1
	}

	// 启动工作线程
	for workerID := 0; workerID < workerCount; workerID++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// 计算此线程的块范围
			startChunk := workerID * chunksPerWorker
			endChunk := (workerID + 1) * chunksPerWorker
			if workerID == workerCount-1 {
				endChunk = chunkCount // 最后一个线程处理所有剩余块
			}

			// 如果有保存的状态，从该位置继续
			if lastProcessed, ok := threadStatus[workerID]; ok && lastProcessed > startChunk {
				startChunk = lastProcessed + 1
			}

			// 用于批量更新的局部变量
			localCompletedCount := int64(0)
			updateBatch := make(map[int]int)

			// 处理分配的块
			for chunkIndex := startChunk; chunkIndex < endChunk; chunkIndex++ {
				// 检查是否被取消
				select {
				case <-task.cancel:
					return
				default:
				}

				// 计算块的字节范围
				start := int64(chunkIndex) * task.ChunkSize
				end := start + task.ChunkSize - 1
				if end >= task.FileSize {
					end = task.FileSize - 1
				}

				// 下载此块
				err := downloadChunk(task.URL, file, start, end)
				if err != nil {
					select {
					case errorCh <- err:
					default:
					}
					return
				}

				// 更新状态
				localCompletedCount++

				// 到达更新批次大小或是最后一块时，批量更新状态
				if localCompletedCount >= 10 || chunkIndex == endChunk-1 {
					if len(updateBatch) > 0 {
						progressMutex.Lock()

						// 更新全局完成数量
						completedChunks += localCompletedCount

						// 更新ChunkStatus
						task.ChunkStatus[workerID] = localCompletedCount

						// 更新线程状态为最大块索引
						var maxChunkIndex int
						for idx := range updateBatch {
							if idx > maxChunkIndex {
								maxChunkIndex = idx
							}
						}
						threadStatus[workerID] = maxChunkIndex

						// 更新进度
						progress := float64(completedChunks) / float64(chunkCount) * 100
						task.Progress = progress

						// 回调通知进度
						if task.OnProgress != nil {
							task.OnProgress(progress)
						}

						// 判断是否需要保存状态
						needSave := completedChunks%10 == 0

						progressMutex.Unlock()

						// 锁外保存状态
						if needSave {
							s.saveTaskWithThreadStatus(task, threadStatus)
						}

						// 重置局部计数器和批次
						localCompletedCount = 0
						updateBatch = make(map[int]int)
					}
				}
			}
		}(workerID)
	}

	// 等待所有块下载完成
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	// 等待完成或错误
	select {
	case err := <-errorCh:
		// 保存状态以便之后恢复
		s.saveTaskWithThreadStatus(task, threadStatus)
		return err
	case <-task.cancel:
		s.saveTaskWithThreadStatus(task, threadStatus)
		return errors.New("任务已取消")
	case <-doneCh:
		// 检查是否全部完成
		allComplete := true
		for _, status := range task.ChunkStatus {
			if status == 0 {
				allComplete = false
				break
			}
		}
		if !allComplete {
			s.saveTaskWithThreadStatus(task, threadStatus)
			return errors.New("下载未完成")
		}

		// 重命名临时文件
		if err := os.Rename(tempFile, task.FilePath); err != nil {
			return err
		}

		task.Completed = true
		task.Progress = 100
		if task.OnProgress != nil {
			task.OnProgress(100)
		}
		return nil
	}
}

// 下载单个块的辅助函数
func downloadChunk(url string, file *os.File, start, end int64) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	// 添加Range头以请求特定的字节范围
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	// 发送请求
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			color.Red("Error closing response body: %v", err)
		}
	}(resp.Body)

	// 验证响应状态
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("意外的状态码: %d", resp.StatusCode)
	}

	// 使用新添加的函数从reader直接写入到文件
	_, err := file.Seek(start, 0)
	if err != nil {
		return err
	}
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		color.Red("Error writing chunk to file: %v", err)
		return err
	}
	if err != nil {
		color.Red("Error writing chunk to file: %v", err)
		return err
	}
	return nil
}

// waitForDownloadCompletion 等待下载完成并处理下载结果
func (s *TransferService) waitForDownloadCompletion(task *FileTask, wg *sync.WaitGroup, errorCh chan error,
	doneCh chan struct{}, tempFile string) error {
	// 等待所有块下载完成或出错
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	// 等待完成或取消
	select {
	case err := <-errorCh:
		return err
	case <-task.cancel:
		return errors.New("任务已取消")
	case <-doneCh:
		// 检查是否所有块都已完成
		if strings.Contains(string(task.ChunkStatus), "0") {
			return errors.New("下载未完成")
		}

		// 重命名临时文件为最终文件
		if err := os.Rename(tempFile, task.FilePath); err != nil {
			return err
		}

		task.Completed = true
		task.Progress = 100
		if task.OnProgress != nil {
			task.OnProgress(100)
		}

		return nil
	}
}

// 处理上传任务
func (s *TransferService) processUpload(task *FileTask) error {
	// 实现文件上传逻辑
	// 这里使用与下载类似的多块上传方法
	// 具体实现会根据你的上传目标服务器API有所不同

	file, err := os.Open(task.FilePath)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			color.Red("Error closing file %s: %v", task.FilePath, err)
		}
	}(file)

	// 计算块数量
	chunkCount := int(math.Ceil(float64(task.FileSize) / float64(task.ChunkSize)))

	// 初始化ChunkStatus，如果为空
	if len(task.ChunkStatus) == 0 {
		task.ChunkStatus = make([]int64, chunkCount)
	}
	// 这里实现根据ChunkStatus中记录已经完成的位置进行计算
	// ...

	return nil
}

// CancelTask 取消任务
func (s *TransferService) CancelTask(taskID string) bool {
	s.jobsMutex.Lock()
	defer s.jobsMutex.Unlock()

	task, exists := s.activeJobs[taskID]
	if !exists {
		return false
	}

	close(task.cancel)
	delete(s.activeJobs, taskID)
	return true
}

func (s *TransferService) saveTaskWithThreadStatus(task *FileTask, status map[int]int) {
	metaFile := filepath.Join(s.config.MetaDir, task.ID+".json")

	metaData := models.TaskMetadata{
		ID:           task.ID,
		CreatedTime:  time.Now(),
		LastModified: time.Now(),
		FilePath:     task.FilePath,
		FileName:     task.FileName,
		TotalSize:    task.FileSize,
		ChunkSize:    task.ChunkSize,
		ChunkStatus:  task.ChunkStatus,
		ThreadStatus: status,
		Progress:     task.Progress,
		Completed:    task.Completed,
		TaskType:     task.TaskType,
		URL:          task.URL,
	}

	jsonData, err := json.MarshalIndent(metaData, "", "  ")
	if err != nil {
		color.Red("Error marshalling task metadata for %s: %v", task.ID, err)
		return
	}

	if err := os.WriteFile(metaFile, jsonData, 0644); err != nil {
		color.Red("Error writing task metadata to %s: %v", metaFile, err)
	}
}
