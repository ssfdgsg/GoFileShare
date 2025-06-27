package services

import (
	"GoFileShare/models"
	"GoFileShare/proto"
	"GoFileShare/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/donnie4w/go-logger/logger"
	"github.com/fatih/color"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
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
	ID             string
	URL            string        // 下载URL或上传目标
	FilePath       string        // 本地文件路径
	FileName       string        // 文件名
	FileSize       int64         // 文件大小
	ChunkSize      int64         // 分块大小
	WorkerProgress map[int]int64 // 核心状态：workerID -> 已完成的块数
	Progress       float64
	Completed      bool
	TaskType       string // "download" 或 "upload"
	OnProgress     func(float64)
	OnComplete     func(*FileTask)
	OnError        func(*FileTask, error)
	cancel         chan struct{}
}

type UploadTask struct {
	filePath string
	fileName string
	url      string
	fileSize int64
}

// NewTransferService 创建传输服务
func NewTransferService(config models.TransferConfig) *TransferService {
	// 确保元数据目录存在
	err := os.MkdirAll(config.MetaDir, 0755)
	if err != nil {
		logger.Error("Error creating meta directory %s: %v", config.MetaDir, err)
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
			logger.Error(err.Error())
			color.Red("Error saving task status: %v", err)
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

// saveTaskStatus 保存任务状态
func (s *TransferService) saveTaskStatus(task *FileTask) error {
	s.jobsMutex.RLock()
	_, ok := s.activeJobs[task.ID]
	s.jobsMutex.RUnlock()
	if !ok && !task.Completed { // 如果任务不在活动列表且未完成（例如，因错误退出），则不保存
		return nil
	}

	metaFile := filepath.Join(s.config.MetaDir, task.ID+".json")
	metaData := models.TaskMetadata{
		ID:             task.ID,
		CreatedTime:    time.Now(), // Can be optimized to store initial time
		LastModified:   time.Now(),
		FilePath:       task.FilePath,
		FileName:       task.FileName,
		TotalSize:      task.FileSize,
		ChunkSize:      task.ChunkSize,
		WorkerProgress: task.WorkerProgress, // 保存新的状态
		Progress:       task.Progress,
		Completed:      task.Completed,
		TaskType:       task.TaskType,
		URL:            task.URL,
	}

	jsonData, err := json.MarshalIndent(metaData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metaFile, jsonData, 0644)
}

// loadTaskState 加载任务状态
func (s *TransferService) loadTaskState(task *FileTask) error {
	metaFile := filepath.Join(s.config.MetaDir, task.ID+".json")
	data, err := os.ReadFile(metaFile)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，初始化一个新的任务状态
			task.WorkerProgress = make(map[int]int64)
			return nil
		}
		return err
	}

	var meta models.TaskMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}

	// 恢复状态
	task.WorkerProgress = meta.WorkerProgress
	if task.WorkerProgress == nil { // 兼容旧的元数据文件
		task.WorkerProgress = make(map[int]int64)
	}
	task.Progress = meta.Progress
	task.Completed = meta.Completed
	return nil
}

// periodicStatusSave 定期保存状态
func (s *TransferService) periodicStatusSave() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.jobsMutex.RLock()
			for _, task := range s.activeJobs {
				// 复制一份以在锁外保存
				taskCopy := *task
				err := s.saveTaskStatus(&taskCopy)
				if err != nil {
					logger.Error("Error saving task status for %s: %v", task.ID, err)
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
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		logger.Error("Error creating directory for %s: %v", filePath, err)
		color.Red("Error creating directory for %s: %s", filePath, err)
		if onError != nil {
			onError(nil, err)
		}
		return ""
	}

	resp, err := http.Head(url)
	if err != nil {
		logger.Error("Error HEADing %s: %v", url, err)
		color.Red("Error HEADing %s: %s", url, err)
		if onError != nil {
			onError(nil, err)
		}
		return ""
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logger.Error("Error closing response body for %s: %v", url, err)
			color.Red("Error closing response body for %s: %v", url, err)
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

	if err := s.loadTaskState(task); err != nil {
		logger.Error("Error loading task state for %s: %v", task.ID, err)
		color.Red("Error loading task state for %s: %v", task.ID, err)
		if onError != nil {
			onError(task, err)
		}
		return ""
	}

	s.jobsMutex.Lock()
	s.activeJobs[task.ID] = task
	s.jobsMutex.Unlock()

	s.workerPool.Submit(func() {
		if err := s.processDownload(task); err != nil {
			if task.OnError != nil {
				task.OnError(task, err)
			}
		} else if task.OnComplete != nil {
			task.OnComplete(task)
		}

		s.jobsMutex.Lock()
		delete(s.activeJobs, task.ID)
		s.jobsMutex.Unlock()
	})

	return task.ID
}

// processDownload 处理下载任务
func (s *TransferService) processDownload(task *FileTask) error {
	// 1. 准备文件
	tempFile := task.FilePath + ".download"
	file, err := os.OpenFile(tempFile, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			logger.Error("Error closing file %s: %v", tempFile, err)
			color.Red("Error closing file %s: %v", tempFile, err)
		}
	}(file)
	if err := file.Truncate(task.FileSize); err != nil {
		return err
	}

	// 2. 计算任务分配
	totalChunkCount := int(math.Ceil(float64(task.FileSize) / float64(task.ChunkSize)))
	workerCount := s.config.WorkerCount
	chunksPerWorker := totalChunkCount / workerCount

	// 3. 初始化并发控制
	var wg sync.WaitGroup
	var progressMutex sync.Mutex
	errorCh := make(chan error, workerCount) // 缓冲通道，防止worker阻塞

	// 4. 启动所有 workers
	for workerID := 0; workerID < workerCount; workerID++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// 计算此 worker 的块范围
			startChunk := workerID * chunksPerWorker
			endChunk := (workerID + 1) * chunksPerWorker
			if workerID == workerCount-1 {
				endChunk = totalChunkCount // 最后一个 worker 处理所有剩余的
			}

			// 恢复逻辑：计算此 worker 真正的起始点
			progressMutex.Lock()
			completedByThisWorker := task.WorkerProgress[workerID]
			progressMutex.Unlock()

			// 从上次完成的地方继续
			resumeStartChunk := startChunk + int(completedByThisWorker)

			if resumeStartChunk < endChunk {
				color.Green("Worker %d: Resuming from chunk %d (Total for this worker: %d to %d)",
					workerID, resumeStartChunk, startChunk, endChunk-1)
			}

			for chunkIndex := resumeStartChunk; chunkIndex < endChunk; chunkIndex++ {
				select {
				case <-task.cancel:
					return // 任务被取消
				default:
				}

				// 下载单个块
				startByte := int64(chunkIndex) * task.ChunkSize
				endByte := startByte + task.ChunkSize - 1
				if endByte >= task.FileSize {
					endByte = task.FileSize - 1
				}

				err := downloadChunk(task.URL, file, startByte, endByte)
				if err != nil {
					select {
					case errorCh <- fmt.Errorf("worker %d failed on chunk %d: %w", workerID, chunkIndex, err):
					default:
					}
					return
				}

				// 更新进度 (在锁内)
				progressMutex.Lock()
				task.WorkerProgress[workerID]++
				var totalCompleted int64
				for _, count := range task.WorkerProgress {
					totalCompleted += count
				}
				task.Progress = float64(totalCompleted) / float64(totalChunkCount) * 100
				currentProgress := task.Progress
				progressMutex.Unlock()

				// 在锁外调用回调
				if task.OnProgress != nil {
					go task.OnProgress(currentProgress) // 异步调用，防止阻塞
				}
			}
		}(workerID)
	}

	// 5. 等待所有 workers 完成
	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	// 6. 等待完成或错误
	select {
	case err := <-errorCh:
		err = s.saveTaskStatus(task)
		if err != nil {
			logger.Error("Error saving task status after error: %v", err)
			color.Red("Error saving task status after error: %v", err)
			return err
		} // 发生错误，保存当前进度
		return err
	case <-task.cancel:
		err = s.saveTaskStatus(task)
		if err != nil {
			logger.Error("Error saving task status on cancel: %v", err)
			color.Red("Error saving task status on cancel: %v", err)
			return err
		}
		return errors.New("任务已取消")
	case <-doneCh:
		// 全部完成，继续执行
	}

	// 7. 最终验证和完成
	var finalCompletedCount int64
	for _, count := range task.WorkerProgress {
		finalCompletedCount += count
	}
	if finalCompletedCount != int64(totalChunkCount) {
		err := s.saveTaskStatus(task)
		if err != nil {
			logger.Error("Error saving task status after final check: %v", err)
			color.Red("Error saving task status after final check: %v", err)
			return err
		}
		return fmt.Errorf("下载未完全完成，预期 %d 块，实际完成 %d 块", totalChunkCount, finalCompletedCount)
	}

	if err := os.Rename(tempFile, task.FilePath); err != nil {
		return err
	}
	task.Completed = true
	task.Progress = 100
	if task.OnProgress != nil {
		task.OnProgress(100)
	}
	// 删除元数据文件
	metaFile := filepath.Join(s.config.MetaDir, task.ID+".json")
	err = os.Remove(metaFile)
	if err != nil {
		logger.Error("Error removing metadata file %s: %v", metaFile, err)
		color.Red("Error removing metadata file %s: %v", metaFile, err)
		return err
	}

	return nil
}

// downloadChunk 下载单个块的辅助函数
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
			logger.Error("Error closing response body: %v", err)
			color.Red("Error closing response body: %v", err)
		}
	}(resp.Body)

	// 验证响应状态
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("意外的状态码: %d", resp.StatusCode)
	}

	// 使用新添加的函数从reader直接写入到文件
	_, err = file.Seek(start, 0)
	if err != nil {
		return err
	}
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		logger.Error("Error writing chunk to file: %v", err)
		color.Red("Error writing chunk to file: %v", err)
		return err
	}
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
	// 不要立即删除，让任务自然退出
	return true
}

type server struct {
	proto.CallUploadServer
}

type client struct {
	proto.CallUploadClient
}

func listenUploadS() {
	// 启动 gRPC 服务器，监听上传任务
	listen, err := net.Listen("tcp", ":18521")
	if err != nil {
		fmt.Printf("failed to listen: %v", err)
		return
	}
	s := grpc.NewServer()
	proto.RegisterCallUploadServer(s, &server{})
	reflection.Register(s)
	defer func() {
		s.Stop()
		err := listen.Close()
		if err != nil {
			logger.Error("Error closing grpc listener: %v", err)
			color.Red("Error closing upload server: %v", err)
			return
		}
	}()
	fmt.Println("Serving 8001...Listen to the file upload task")
	err = s.Serve(listen)
	if err != nil {
		fmt.Printf("failed to serve: %v", err)
		return
	}
}

// callUploads 调用上传函数(Server端)
func (s *server) callUploadS(fileInfo *proto.FileInfo) (*proto.ErrInfo, error) {
	// 实现上传逻辑 x
	var uploadTask UploadTask
	err := json.Unmarshal([]byte(fileInfo.FileDataJson), &uploadTask)
	if err != nil {
		logger.Error("Error unmarshalling upload task: %v", err)
		color.Red("Error unmarshalling upload task: %v", err)
		return nil, err
	}
	transferService := NewTransferService(models.TransferConfig{
		MetaDir:     "meta",      // 元数据目录
		WorkerCount: 4,           // 并发 worker 数量
		ChunkSize:   1024 * 1024, // 每块大小，单位：字节
	})
	if transferService == nil {
		return &proto.ErrInfo{ErrStr: "Failed to initialize TransferService"}, errors.New("TransferService initialization failed")
	}
	taskID := transferService.AddDownloadTask(uploadTask.url, uploadTask.filePath, onProgress, onComplete, onError)
	color.Green("Upload task %v successfully", taskID)
	return &proto.ErrInfo{ErrStr: ""}, nil
}

// callUploadC 调用上传函数(Client端)
func callUploadC(task UploadTask, serviceHost string) {
	conn, err := grpc.NewClient(serviceHost, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Println(err)
	}
	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {
			logger.Error("Error closing connection: %v", err)
			color.Red("Error closing connection: %v", err)
		}
	}(conn)
	taskJson := MakeTaskShareJSON(task.filePath, task.url, task.fileName, task.fileSize)
	client := proto.NewCallUploadClient(conn)
	_, err = client.CallUpload(context.Background(), &proto.FileInfo{FileDataJson: taskJson})
	if err != nil {
		logger.Error("Error calling upload: %v", err)
		color.Red("Error saving task status after error: %v", err)
		return
	}
	color.Green("Upload task %v successfully", task.fileName)
}

// MakeTaskShareJSON 创建用来发送上传任务的JSON
func MakeTaskShareJSON(filePath, url, fileName string, fileSize int64) []byte {
	task := UploadTask{fileName: fileName, filePath: filePath, url: url, fileSize: fileSize}
	outJson, err := json.Marshal(task)
	if err != nil {
		logger.Error("Error marshalling task json: %v", err)
		color.Red("Error marshalling task json: %v", err)
	}
	return outJson
}

func onProgress(Progress float64) {

	color.Green("Task progress: %.2f%%", Progress)

}

func onComplete(task *FileTask) {
	color.Green("Task %s completed successfully! File saved to %s", task.ID, task.FilePath)
}

func onError(task *FileTask, err error) {
	logger.Error("Task %s encountered an error: %v", task.ID, err)
	logger.Error("Error saving task status to file %s: %v", task.FileName, err)
	color.Red("Task %s encountered an error: %v", task.ID, err)
	color.Red("Error saving task status to file %s: %v", task.FileName, err)
}
