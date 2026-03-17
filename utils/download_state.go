package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/donnie4w/go-logger/logger"
	"github.com/fatih/color"
)

// DownloadState 下载状态结构
type DownloadState struct {
	FileID       string    `json:"file_id"`       // 文件唯一标识
	FileName     string    `json:"file_name"`     // 文件名
	FilePath     string    `json:"file_path"`     // 本地文件路径
	FileSize     int64     `json:"file_size"`     // 文件总大小
	DownloadURL  string    `json:"download_url"`  // 下载URL
	Downloaded   int64     `json:"downloaded"`    // 已下载字节数
	Status       string    `json:"status"`        // 下载状态: pending, downloading, paused, completed, failed
	CreatedAt    time.Time `json:"created_at"`    // 创建时间
	UpdatedAt    time.Time `json:"updated_at"`    // 更新时间
	MD5Hash      string    `json:"md5_hash"`      // 文件MD5哈希（可选）
	ChunkSize    int64     `json:"chunk_size"`    // 分块大小
	Chunks       []Chunk   `json:"chunks"`        // 分块信息
}

// Chunk 分块信息
type Chunk struct {
	Index      int   `json:"index"`       // 分块索引
	Start      int64 `json:"start"`       // 起始位置
	End        int64 `json:"end"`         // 结束位置
	Downloaded bool  `json:"downloaded"`  // 是否已下载
}

// DownloadStateManager 下载状态管理器
type DownloadStateManager struct {
	stateDir string
	mutex    sync.RWMutex
	states   map[string]*DownloadState
}

// NewDownloadStateManager 创建下载状态管理器
func NewDownloadStateManager(stateDir string) *DownloadStateManager {
	if stateDir == "" {
		stateDir = filepath.Join(os.TempDir(), "download_states")
	}
	
	// 确保状态目录存在
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		logger.Errorf("Failed to create state directory %s: %v", stateDir, err)
		color.Red("Failed to create state directory %s: %v", stateDir, err)
	}
	
	manager := &DownloadStateManager{
		stateDir: stateDir,
		states:   make(map[string]*DownloadState),
	}
	
	// 加载现有状态
	manager.loadAllStates()
	
	return manager
}

// CreateDownloadState 创建新的下载状态
func (dsm *DownloadStateManager) CreateDownloadState(fileID, fileName, filePath, downloadURL string, fileSize int64, chunkSize int64) *DownloadState {
	dsm.mutex.Lock()
	defer dsm.mutex.Unlock()
	
	now := time.Now()
	
	// 计算分块数量
	chunkCount := (fileSize + chunkSize - 1) / chunkSize
	chunks := make([]Chunk, chunkCount)
	
	for i := int64(0); i < chunkCount; i++ {
		start := i * chunkSize
		end := start + chunkSize - 1
		if end >= fileSize {
			end = fileSize - 1
		}
		
		chunks[i] = Chunk{
			Index:      int(i),
			Start:      start,
			End:        end,
			Downloaded: false,
		}
	}
	
	state := &DownloadState{
		FileID:      fileID,
		FileName:    fileName,
		FilePath:    filePath,
		FileSize:    fileSize,
		DownloadURL: downloadURL,
		Downloaded:  0,
		Status:      "pending",
		CreatedAt:   now,
		UpdatedAt:   now,
		ChunkSize:   chunkSize,
		Chunks:      chunks,
	}
	
	dsm.states[fileID] = state
	dsm.saveState(state)
	
	color.Green("Created download state for file: %s", fileName)
	return state
}

// GetDownloadState 获取下载状态
func (dsm *DownloadStateManager) GetDownloadState(fileID string) (*DownloadState, bool) {
	dsm.mutex.RLock()
	defer dsm.mutex.RUnlock()
	
	state, exists := dsm.states[fileID]
	return state, exists
}

// UpdateDownloadProgress 更新下载进度
func (dsm *DownloadStateManager) UpdateDownloadProgress(fileID string, chunkIndex int, downloaded int64) error {
	dsm.mutex.Lock()
	defer dsm.mutex.Unlock()
	
	state, exists := dsm.states[fileID]
	if !exists {
		return fmt.Errorf("download state not found for file ID: %s", fileID)
	}
	
	// 更新分块状态
	if chunkIndex >= 0 && chunkIndex < len(state.Chunks) {
		if !state.Chunks[chunkIndex].Downloaded {
			state.Chunks[chunkIndex].Downloaded = true
			state.Downloaded += downloaded
		}
	}
	
	state.UpdatedAt = time.Now()
	
	// 检查是否完成
	allCompleted := true
	for _, chunk := range state.Chunks {
		if !chunk.Downloaded {
			allCompleted = false
			break
		}
	}
	
	if allCompleted {
		state.Status = "completed"
		color.Green("Download completed for file: %s", state.FileName)
	}
	
	return dsm.saveState(state)
}

// UpdateDownloadStatus 更新下载状态
func (dsm *DownloadStateManager) UpdateDownloadStatus(fileID, status string) error {
	dsm.mutex.Lock()
	defer dsm.mutex.Unlock()
	
	state, exists := dsm.states[fileID]
	if !exists {
		return fmt.Errorf("download state not found for file ID: %s", fileID)
	}
	
	state.Status = status
	state.UpdatedAt = time.Now()
	
	return dsm.saveState(state)
}

// GetIncompleteDownloads 获取未完成的下载
func (dsm *DownloadStateManager) GetIncompleteDownloads() []*DownloadState {
	dsm.mutex.RLock()
	defer dsm.mutex.RUnlock()
	
	var incomplete []*DownloadState
	for _, state := range dsm.states {
		if state.Status != "completed" {
			incomplete = append(incomplete, state)
		}
	}
	
	return incomplete
}

// RemoveDownloadState 删除下载状态
func (dsm *DownloadStateManager) RemoveDownloadState(fileID string) error {
	dsm.mutex.Lock()
	defer dsm.mutex.Unlock()
	
	delete(dsm.states, fileID)
	
	stateFile := filepath.Join(dsm.stateDir, fileID+".json")
	if err := os.Remove(stateFile); err != nil && !os.IsNotExist(err) {
		logger.Errorf("Failed to remove state file %s: %v", stateFile, err)
		return err
	}
	
	color.Yellow("Removed download state for file ID: %s", fileID)
	return nil
}

// saveState 保存状态到文件
func (dsm *DownloadStateManager) saveState(state *DownloadState) error {
	stateFile := filepath.Join(dsm.stateDir, state.FileID+".json")
	
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		logger.Errorf("Failed to marshal download state: %v", err)
		return err
	}
	
	if err := os.WriteFile(stateFile, data, 0644); err != nil {
		logger.Errorf("Failed to save state file %s: %v", stateFile, err)
		return err
	}
	
	return nil
}

// loadAllStates 加载所有状态文件
func (dsm *DownloadStateManager) loadAllStates() {
	files, err := filepath.Glob(filepath.Join(dsm.stateDir, "*.json"))
	if err != nil {
		logger.Errorf("Failed to list state files: %v", err)
		return
	}
	
	for _, file := range files {
		state, err := dsm.loadState(file)
		if err != nil {
			logger.Errorf("Failed to load state file %s: %v", file, err)
			continue
		}
		
		dsm.states[state.FileID] = state
	}
	
	color.Green("Loaded %d download states", len(dsm.states))
}

// loadState 从文件加载状态
func (dsm *DownloadStateManager) loadState(stateFile string) (*DownloadState, error) {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return nil, err
	}
	
	var state DownloadState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	
	return &state, nil
}

// GetDownloadProgress 获取下载进度百分比
func (dsm *DownloadStateManager) GetDownloadProgress(fileID string) float64 {
	dsm.mutex.RLock()
	defer dsm.mutex.RUnlock()
	
	state, exists := dsm.states[fileID]
	if !exists || state.FileSize == 0 {
		return 0
	}
	
	return float64(state.Downloaded) / float64(state.FileSize) * 100
}

// ListAllDownloads 列出所有下载状态
func (dsm *DownloadStateManager) ListAllDownloads() []*DownloadState {
	dsm.mutex.RLock()
	defer dsm.mutex.RUnlock()
	
	var downloads []*DownloadState
	for _, state := range dsm.states {
		downloads = append(downloads, state)
	}
	
	return downloads
}