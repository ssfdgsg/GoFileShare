package services

import (
	"GoFileShare/utils"
	"github.com/fatih/color"
	"net/http"
	"os"
	"sync"
	"time"
)

type TransferTask interface {
	Execute() error
	Cancel()
	GetID() string
	GetProgress() float64
	SaveState() error // 保存传输状态
	LoadState() error // 加载传输状态
	Resume() error    // 恢复传输
}

// 传输任务元数据
type TaskMetadata struct {
	ID           string    // 任务标识符
	CreatedTime  time.Time // 创建时间
	LastModified time.Time // 最后修改时间
	ChunkSize    int64     // 块大小
	TotalSize    int64     // 总大小
	ChunkStatus  []bool    // 块状态
	Progress     float64   // 当前进度
	Completed    bool      // 是否完成
	Type         string    // download/upload
}

type DownloadService struct {
	workerPool     *utils.WorkerPool
	tasks          chan *DownloadTask
	activeJobs     map[string]*DownloadTask
	jobsMutex      sync.RWMutex
	metaDir        string        // 保存元数据的目录
	statusInterval time.Duration // 状态保存间隔
	stopCh         chan struct{}
}

// 创建下载服务
func NewDownloadService(workerCount int, metaDir string) *DownloadService {
	// 确保元数据目录存在
	os.MkdirAll(metaDir, 0755)

	return &DownloadService{
		workerPool:     utils.NewWorkerPool(workerCount),
		tasks:          make(chan *DownloadTask, 100),
		activeJobs:     make(map[string]*DownloadTask),
		metaDir:        metaDir,
		statusInterval: 5 * time.Second,
		stopCh:         make(chan struct{}),
	}
}

// 下载任务定义
type DownloadTask struct {
	ID          string
	URL         string
	FilePath    string
	FileName    string
	FileSize    int64
	ChunkSize   int64
	ChunkStatus []bool  // 每个分块的完成状态
	Progress    float64 // 0-100
	Completed   bool
	OnProgress  func(float64)
	OnComplete  func(*DownloadTask)
	OnError     func(*DownloadTask, error)
}

// 上传任务定义
type UploadTask struct {
	ID          string
	FilePath    string
	FileName    string
	TargetURL   string
	FileSize    int64
	ChunkSize   int64
	ChunkStatus []bool  // 每个分块的完成状态
	Progress    float64 // 0-100
	Completed   bool
	OnProgress  func(float64)
	OnComplete  func(*UploadTask)
	OnError     func(*UploadTask, error)
}
