// models/transfer.go
package models

import "time"

// TransferConfig 传输配置
type TransferConfig struct {
	WorkerCount int    // 工作协程数量
	MetaDir     string // 元数据保存目录
	ChunkSize   int64  // 分块大小
}

// TaskMetadata 任务元数据
type TaskMetadata struct {
	ID             string        // 任务ID
	CreatedTime    time.Time     // 创建时间
	LastModified   time.Time     // 最后修改时间
	FilePath       string        // 文件路径
	FileName       string        // 文件名
	TotalSize      int64         // 总文件大小
	ChunkSize      int64         // 分块大小
	WorkerProgress map[int]int64 // 每个线程的进度(线程ID -> 最后处理的块索引)
	Progress       float64       // 进度百分比
	Completed      bool          // 是否完成
	TaskType       string        // 任务类型："download"或"upload"
	URL            string        // 下载URL或上传目标
}

// TransferTask 文件传输任务接口
type TransferTask interface {
	Execute() error       // 执行任务
	Cancel()              // 取消任务
	GetID() string        // 获取任务ID
	GetProgress() float64 // 获取进度
	SaveState() error     // 保存状态
	Resume() error        // 恢复任务
}
