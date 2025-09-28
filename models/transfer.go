// models/transfer.go
package models

import "time"

// APIResponse 统一API响应格式
type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// HolePunchInfo 打洞请求信息
type HolePunchInfo struct {
	HasRequest   bool   `json:"has_request"`
	RequesterKey string `json:"requester_key,omitempty"`
	ExternalIP   string `json:"external_ip,omitempty"`
	ExternalPort string `json:"external_port,omitempty"`
	LocalIP      string `json:"local_ip,omitempty"`
	LocalPort    string `json:"local_port,omitempty"`
	Timestamp    int64  `json:"timestamp,omitempty"`
}

// TargetClientInfo 目标客户端信息
type TargetClientInfo struct {
	ExternalIP   string // 目标客户端外网IP
	ExternalPort string // 目标客户端外网端口
}

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
	URL            string        // 下载URL或上���目标
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
