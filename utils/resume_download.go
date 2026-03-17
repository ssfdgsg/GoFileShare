package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/donnie4w/go-logger/logger"
	"github.com/fatih/color"
)

// ResumableDownloader 可断点续传的下载器
type ResumableDownloader struct {
	stateManager *DownloadStateManager
	client       *http.Client
	maxRetries   int
	retryDelay   time.Duration
}

// NewResumableDownloader 创建新的可断点续传下载器
func NewResumableDownloader(stateDir string) *ResumableDownloader {
	return &ResumableDownloader{
		stateManager: NewDownloadStateManager(stateDir),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		maxRetries: 3,
		retryDelay: 2 * time.Second,
	}
}

// StartDownload 开始下载文件
func (rd *ResumableDownloader) StartDownload(fileID, fileName, downloadURL, savePath string, chunkSize int64) error {
	// 检查是否已存在下载状态
	if state, exists := rd.stateManager.GetDownloadState(fileID); exists {
		color.Yellow("Found existing download state for file: %s", fileName)
		return rd.resumeDownload(state)
	}
	
	// 获取文件大小
	fileSize, err := rd.getFileSize(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to get file size: %v", err)
	}
	
	// 确保保存目录存在
	if err := os.MkdirAll(filepath.Dir(savePath), 0755); err != nil {
		return fmt.Errorf("failed to create save directory: %v", err)
	}
	
	// 创建下载状态
	state := rd.stateManager.CreateDownloadState(fileID, fileName, savePath, downloadURL, fileSize, chunkSize)
	
	color.Green("Starting download: %s (%.2f MB)", fileName, float64(fileSize)/(1024*1024))
	
	return rd.downloadFile(state)
}

// ResumeDownload 恢复下载
func (rd *ResumableDownloader) ResumeDownload(fileID string) error {
	state, exists := rd.stateManager.GetDownloadState(fileID)
	if !exists {
		return fmt.Errorf("download state not found for file ID: %s", fileID)
	}
	
	if state.Status == "completed" {
		color.Green("File already completed: %s", state.FileName)
		return nil
	}
	
	color.Yellow("Resuming download: %s", state.FileName)
	return rd.resumeDownload(state)
}

// ResumeAllIncompleteDownloads 恢复所有未完成的下载
func (rd *ResumableDownloader) ResumeAllIncompleteDownloads() error {
	incompleteDownloads := rd.stateManager.GetIncompleteDownloads()
	
	if len(incompleteDownloads) == 0 {
		color.Green("No incomplete downloads found")
		return nil
	}
	
	color.Yellow("Found %d incomplete downloads, resuming...", len(incompleteDownloads))
	
	var wg sync.WaitGroup
	errChan := make(chan error, len(incompleteDownloads))
	
	for _, state := range incompleteDownloads {
		wg.Add(1)
		go func(s *DownloadState) {
			defer wg.Done()
			if err := rd.resumeDownload(s); err != nil {
				errChan <- fmt.Errorf("failed to resume %s: %v", s.FileName, err)
			}
		}(state)
	}
	
	wg.Wait()
	close(errChan)
	
	// 收集错误
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}
	
	if len(errors) > 0 {
		for _, err := range errors {
			logger.Errorf("Resume error: %v", err)
			color.Red("Resume error: %v", err)
		}
		return fmt.Errorf("failed to resume %d downloads", len(errors))
	}
	
	color.Green("All incomplete downloads resumed successfully")
	return nil
}

// resumeDownload 恢复下载
func (rd *ResumableDownloader) resumeDownload(state *DownloadState) error {
	// 更新状态为下载中
	if err := rd.stateManager.UpdateDownloadStatus(state.FileID, "downloading"); err != nil {
		return err
	}
	
	return rd.downloadFile(state)
}

// downloadFile 下载文件
func (rd *ResumableDownloader) downloadFile(state *DownloadState) error {
	// 创建或打开目标文件
	file, err := os.OpenFile(state.FilePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		rd.stateManager.UpdateDownloadStatus(state.FileID, "failed")
		return fmt.Errorf("failed to create/open file: %v", err)
	}
	defer file.Close()
	
	// 预分配文件大小
	if err := file.Truncate(state.FileSize); err != nil {
		logger.Warnf("Failed to truncate file to size %d: %v", state.FileSize, err)
	}
	
	// 并发下载未完成的分块
	var wg sync.WaitGroup
	errChan := make(chan error, len(state.Chunks))
	semaphore := make(chan struct{}, 5) // 限制并发数为5
	
	for i, chunk := range state.Chunks {
		if chunk.Downloaded {
			continue // 跳过已下载的分块
		}
		
		wg.Add(1)
		go func(chunkIndex int, c Chunk) {
			defer wg.Done()
			
			semaphore <- struct{}{} // 获取信号量
			defer func() { <-semaphore }() // 释放信号量
			
			if err := rd.downloadChunk(state, file, chunkIndex, c); err != nil {
				errChan <- fmt.Errorf("chunk %d failed: %v", chunkIndex, err)
			}
		}(i, chunk)
	}
	
	wg.Wait()
	close(errChan)
	
	// 检查是否有错误
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}
	
	if len(errors) > 0 {
		rd.stateManager.UpdateDownloadStatus(state.FileID, "failed")
		for _, err := range errors {
			logger.Errorf("Download error: %v", err)
		}
		return fmt.Errorf("download failed with %d errors", len(errors))
	}
	
	// 验证文件完整性
	if err := rd.verifyDownload(state); err != nil {
		rd.stateManager.UpdateDownloadStatus(state.FileID, "failed")
		return fmt.Errorf("download verification failed: %v", err)
	}
	
	color.Green("Download completed successfully: %s", state.FileName)
	return nil
}

// downloadChunk 下载单个分块
func (rd *ResumableDownloader) downloadChunk(state *DownloadState, file *os.File, chunkIndex int, chunk Chunk) error {
	var lastErr error
	
	for retry := 0; retry <= rd.maxRetries; retry++ {
		if retry > 0 {
			color.Yellow("Retrying chunk %d (attempt %d/%d)", chunkIndex, retry, rd.maxRetries)
			time.Sleep(rd.retryDelay)
		}
		
		// 创建HTTP请求
		req, err := http.NewRequest("GET", state.DownloadURL, nil)
		if err != nil {
			lastErr = err
			continue
		}
		
		// 设置Range头进行分块下载
		rangeHeader := fmt.Sprintf("bytes=%d-%d", chunk.Start, chunk.End)
		req.Header.Set("Range", rangeHeader)
		
		// 发送请求
		resp, err := rd.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		
		// 检查状态码
		if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			continue
		}
		
		// 读取数据并写入文件
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		
		if err != nil {
			lastErr = err
			continue
		}
		
		// 写入文件的指定位置
		if _, err := file.WriteAt(data, chunk.Start); err != nil {
			lastErr = err
			continue
		}
		
		// 更新下载进度
		if err := rd.stateManager.UpdateDownloadProgress(state.FileID, chunkIndex, int64(len(data))); err != nil {
			logger.Warnf("Failed to update progress: %v", err)
		}
		
		// 显示进度
		progress := rd.stateManager.GetDownloadProgress(state.FileID)
		color.Cyan("Progress: %.2f%% - Chunk %d completed", progress, chunkIndex)
		
		return nil // 成功
	}
	
	return fmt.Errorf("chunk download failed after %d retries: %v", rd.maxRetries, lastErr)
}

// getFileSize 获取文件大小
func (rd *ResumableDownloader) getFileSize(url string) (int64, error) {
	resp, err := rd.client.Head(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	contentLength := resp.Header.Get("Content-Length")
	if contentLength == "" {
		return 0, fmt.Errorf("content-length header not found")
	}
	
	size, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid content-length: %v", err)
	}
	
	return size, nil
}

// verifyDownload 验证下载完整性
func (rd *ResumableDownloader) verifyDownload(state *DownloadState) error {
	// 检查文件大小
	fileInfo, err := os.Stat(state.FilePath)
	if err != nil {
		return fmt.Errorf("failed to stat downloaded file: %v", err)
	}
	
	if fileInfo.Size() != state.FileSize {
		return fmt.Errorf("file size mismatch: expected %d, got %d", state.FileSize, fileInfo.Size())
	}
	
	// 如果有MD5哈希，进行验证
	if state.MD5Hash != "" {
		actualHash := MD5Check(state.FilePath)
		if actualHash != state.MD5Hash {
			return fmt.Errorf("MD5 hash mismatch: expected %s, got %s", state.MD5Hash, actualHash)
		}
		color.Green("MD5 verification passed")
	}
	
	return nil
}

// GetDownloadProgress 获取下载进度
func (rd *ResumableDownloader) GetDownloadProgress(fileID string) float64 {
	return rd.stateManager.GetDownloadProgress(fileID)
}

// ListDownloads 列出所有下载
func (rd *ResumableDownloader) ListDownloads() []*DownloadState {
	return rd.stateManager.ListAllDownloads()
}

// PauseDownload 暂停下载
func (rd *ResumableDownloader) PauseDownload(fileID string) error {
	return rd.stateManager.UpdateDownloadStatus(fileID, "paused")
}

// CancelDownload 取消下载并删除状态
func (rd *ResumableDownloader) CancelDownload(fileID string) error {
	// 删除部分下载的文件
	if state, exists := rd.stateManager.GetDownloadState(fileID); exists {
		if err := os.Remove(state.FilePath); err != nil && !os.IsNotExist(err) {
			logger.Warnf("Failed to remove partial file %s: %v", state.FilePath, err)
		}
	}
	
	return rd.stateManager.RemoveDownloadState(fileID)
}

// SetMD5Hash 设置文件的MD5哈希用于验证
func (rd *ResumableDownloader) SetMD5Hash(fileID, md5Hash string) error {
	state, exists := rd.stateManager.GetDownloadState(fileID)
	if !exists {
		return fmt.Errorf("download state not found for file ID: %s", fileID)
	}
	
	state.MD5Hash = md5Hash
	return rd.stateManager.saveState(state)
}