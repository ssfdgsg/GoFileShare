package domain

import "time"

// APIResponse defines the common API response shape used by P2P endpoints.
type APIResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// HolePunchInfo represents a hole-punch request payload.
type HolePunchInfo struct {
	HasRequest   bool   `json:"has_request"`
	RequesterKey string `json:"requester_key,omitempty"`
	ExternalIP   string `json:"external_ip,omitempty"`
	ExternalPort string `json:"external_port,omitempty"`
	LocalIP      string `json:"local_ip,omitempty"`
	LocalPort    string `json:"local_port,omitempty"`
	Timestamp    int64  `json:"timestamp,omitempty"`
}

// TargetClientInfo holds remote peer info.
type TargetClientInfo struct {
	ExternalIP   string // remote peer public IP
	ExternalPort string // remote peer public port
}

// TransferConfig represents transfer service configuration.
type TransferConfig struct {
	WorkerCount int    // number of workers
	MetaDir     string // metadata directory
	ChunkSize   int64  // chunk size in bytes
}

// TaskMetadata stores resumable transfer task state.
type TaskMetadata struct {
	ID             string        // task ID
	CreatedTime    time.Time     // created time
	LastModified   time.Time     // last modified time
	FilePath       string        // file path
	FileName       string        // file name
	TotalSize      int64         // total file size
	ChunkSize      int64         // chunk size
	WorkerProgress map[int]int64 // workerID -> last completed chunk index
	Progress       float64       // progress percent
	Completed      bool          // completed state
	TaskType       string        // "download" or "upload"
	URL            string        // download URL or upload target
}

// TransferTask defines resumable transfer behavior.
type TransferTask interface {
	Execute() error       // run task
	Cancel()              // cancel task
	GetID() string        // task ID
	GetProgress() float64 // progress percent
	SaveState() error     // persist state
	Resume() error        // resume task
}
