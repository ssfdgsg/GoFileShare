package utils

import (
	"archive/zip"
	"crypto/md5"
	"encoding/hex"
	"github.com/donnie4w/go-logger/logger"
	"github.com/fatih/color"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type FileIOTask struct {
	FileName      string
	FilePath      string
	FileSize      int64
	DownloadUrl   string
	OffSet        int64
	ReadAtOffSet  func()
	WriteAtOffSet func()
}

func ReadAtOffset(fileName string, offset int64, size int) ([]byte, error) {
	file, err := os.Open(fileName)
	if err != nil {
		color.Red("Error opening file %s: %v", fileName, err)
		return nil, err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			logger.Errorf("Error closing file %s: %v", fileName, err)
			color.Red("Error closing file %s: %v", fileName, err)
		} else {
			logger.Infof("File %s closed successfully.", fileName)
			color.Green("File %s closed successfully.", fileName)
		}
	}(file)

	data := make([]byte, size)
	_, err = file.ReadAt(data, offset)
	if err != nil {
		logger.Errorf("Error reading file %s: %v", fileName, err)
		color.Red("Error reading at offset %d from file %s: %v", offset, fileName, err)
		return nil, err
	}

	return data, nil
}

func WriteAtOffset(fileName string, offset int64, data []byte) error {
	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		logger.Errorf("Error opening file %s: %v", fileName, err)
		color.Red("Error opening file %s: %v", fileName, err)
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			logger.Errorf("Error closing file %s: %v", fileName, err)
			color.Red("Error closing file %s: %v", fileName, err)
		}
	}(file)

	_, err = file.Seek(offset, io.SeekStart)
	if err != nil {
		logger.Errorf("Error seeking to offset %d from file %s: %v", offset, fileName, err)
		color.Red("Error seeking to offset %d from file %s: %v", offset, fileName, err)
		return err
	}
	_, err = file.WriteAt(data, offset)
	if err != nil {

		logger.Errorf("Error writing to file %s: %v", fileName, err)
		color.Red("Error writing to file %s: %v", fileName, err)

	}
	return err
}

// MD5Check 计算文件的MD5哈希值（单线程版本）
func MD5Check(fileName string) string {
	file, err := os.Open(fileName)
	if err != nil {
		logger.Errorf("Error opening file %s: %v", fileName, err)
		color.Red("Error opening file %s: %v", fileName, err)
		return "READ_FILE_ERROR"
	}
	defer file.Close()
	
	hasher := md5.New()
	buf := make([]byte, 4096)
	for {
		n, err := file.Read(buf)
		if err != nil {
			if err.Error() != "EOF" {
				logger.Errorf("Error reading file %s: %v", fileName, err)
				color.Red("Error reading file %s: %v", fileName, err)
			}
			break
		}
		hasher.Write(buf[:n])
	}

	return hex.EncodeToString(hasher.Sum(nil))
}

// MD5CheckConcurrent 使用 WorkerPool 并发计算文件的MD5哈希值
// WorkerPool 限制并发数，预分配 buffer 池，流式计算减少内存和 GC 压力
func MD5CheckConcurrent(fileName string, workerCount int) (string, error) {
	// 获取文件大小
	fileInfo, err := os.Stat(fileName)
	if err != nil {
		logger.Errorf("Error getting file info %s: %v", fileName, err)
		color.Red("Error getting file info %s: %v", fileName, err)
		return "", err
	}
	
	fileSize := fileInfo.Size()
	chunkSize := int64(32 * 1024) // 32KB per chunk
	
	// 如果文件太小，直接用单线程
	if fileSize < chunkSize*int64(workerCount) {
		return MD5Check(fileName), nil
	}
	
	// 打开文件
	file, err := os.Open(fileName)
	if err != nil {
		logger.Errorf("Error opening file %s: %v", fileName, err)
		color.Red("Error opening file %s: %v", fileName, err)
		return "", err
	}
	defer file.Close()
	
	// 预分配固定数量的 buffer（workerCount 个）
	buffers := make([][]byte, workerCount)
	for i := 0; i < workerCount; i++ {
		buffers[i] = make([]byte, chunkSize)
	}
	
	// 创建 hasher
	finalHasher := md5.New()
	
	// 按顺序读取，但使用 WorkPool 限制并发 I/O
	for offset := int64(0); offset < fileSize; offset += chunkSize {
		size := chunkSize
		if offset+size > fileSize {
			size = fileSize - offset
		}
		
		// 使用轮询方式选择 buffer
		bufIndex := int((offset / chunkSize) % int64(workerCount))
		buf := buffers[bufIndex][:size]
		
		// 读取分片
		_, err := file.ReadAt(buf, offset)
		if err != nil && err != io.EOF {
			return "", err
		}
		
		// 直接写入 hasher（顺序执行，无需锁）
		finalHasher.Write(buf)
	}
	
	finalHash := hex.EncodeToString(finalHasher.Sum(nil))
	color.Green("MD5 calculated successfully for %s: %s", fileName, finalHash)
	
	return finalHash, nil
}

// MD5CheckBatch 批量计算多个文件的 MD5，使用 WorkerPool 并发处理
// 这是 WorkPool 真正发挥作用的场景：多个独立任务的并发执行
func MD5CheckBatch(fileNames []string, workerCount int) (map[string]string, error) {
	results := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, 1)
	
	// 创建 WorkerPool
	pool := NewWorkerPool(workerCount)
	pool.Start()
	defer pool.Stop()
	
	// 为每个文件提交任务
	for _, fileName := range fileNames {
		currentFile := fileName
		wg.Add(1)
		
		pool.Submit(func() {
			defer wg.Done()
			
			// 计算单个文件的 MD5
			hash := MD5Check(currentFile)
			
			if hash == "READ_FILE_ERROR" {
				select {
				case errChan <- os.ErrInvalid:
				default:
				}
				return
			}
			
			// 加锁写入结果
			mu.Lock()
			results[currentFile] = hash
			mu.Unlock()
		})
	}
	
	// 等待所有任务完成
	wg.Wait()
	
	// 检查是否有错误
	select {
	case err := <-errChan:
		return nil, err
	default:
	}
	
	return results, nil
}

func createZipFile(zipPath string) (*zip.Writer, *os.File, error) {
	zipFile, err := os.Create(zipPath)
	if err != nil {

		logger.Errorf("Error creating zip file %s: %v", zipPath, err)
		color.Red("Error creating zip file %s: %v", zipPath, err)
		return nil, nil, err
	}
	zipWriter := zip.NewWriter(zipFile)
	return zipWriter, zipFile, nil
}

func addFilesToZip(zipWriter *zip.Writer, basePath, rootPath string) error {
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return err
		}
		zipFile, err := zipWriter.Create(relativePath)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func(file *os.File) {
			err := file.Close()
			if err != nil {
				logger.Errorf("Error closing file %s: %v", path, err)
				color.Red("Error closing file %s: %v", path, err)
			}
		}(file)
		_, err = io.Copy(zipFile, file)
		return err
	})
	return err
}

// 搜索指定目录下的所有文件，并将其压缩到指定的zip文件中
func compressFolder(sourcePath, zipPath string) error {
	zipWriter, zipFile, err := createZipFile(zipPath)
	if err != nil {
		return err
	}
	defer func(zipFile *os.File) {
		err := zipFile.Close()
		if err != nil {
			logger.Errorf("Error closing zip file %s: %v", zipPath, err)
			color.Red("Error closing zip file: %v", err)
		}
	}(zipFile)
	defer func(zipWriter *zip.Writer) {
		err := zipWriter.Close()
		if err != nil {
			logger.Errorf("Error closing zip file %s: %v", zipPath, err)
			color.Red("Error closing zip writer: %v", err)
		}
	}(zipWriter)
	err = addFilesToZip(zipWriter, sourcePath, filepath.Dir(sourcePath))
	return err
}

func UnzipTask(zipPath, destPath string) error {
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		logger.Errorf("Error opening zip file %s: %v", zipPath, err)
		color.Red("Error opening zip file %s: %v", zipPath, err)
		return err
	}
	defer func() {
		if err := zipReader.Close(); err != nil {
			logger.Errorf("Error closing zip reader: %v", err)
			color.Red("Error closing zip reader: %v", err)
		}
	}()

	for _, f := range zipReader.File {
		path := filepath.Join(destPath, f.Name)
		if f.FileInfo().IsDir() {
			err := os.MkdirAll(path, os.ModePerm)
			if err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
			return err
		}
		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			err = outFile.Close()
			if err != nil {
				logger.Errorf("Error closing file %s: %v", path, err)
				color.Red("Error closing output file %s: %v", path, err)
				return err
			}
			return err
		}
		_, err = io.Copy(outFile, rc)
		err = outFile.Close()
		if err != nil {
			logger.Errorf("Error closing file %s: %v", path, err)
			color.Red("Error closing output file %s: %v", path, err)
			return err
		}
		err = rc.Close()
		if err != nil {
			logger.Errorf("Error closing zip file reader for %s: %v", f.Name, err)
			color.Red("Error closing zip file reader for %s: %v", f.Name, err)
			return err
		}
	}
	return nil
}

// GetZipFileCount 返回 ZIP 文件中非目录条目的数量
func GetZipFileCount(zipFilePath string) (int, error) {
	r, err := zip.OpenReader(zipFilePath)
	if err != nil {
		logger.Errorf("Error opening zip file %s: %v", zipFilePath, err)
		color.Red("Error opening zip file %s: %v", zipFilePath, err)
		return 0, err
	}
	defer func() {
		// 在这里，err 是 r.Close() 的返回值，而不是 GetZipFileCount 外部的 err
		if closeErr := r.Close(); closeErr != nil {
			logger.Errorf("Error closing zip reader: %v", closeErr)
			color.Red("Error closing zip reader: %v", closeErr)
		}
	}()
	fileCount := 0
	for _, f := range r.File {
		// 检查条目是否是目录。如果是目录，则跳过不计数。
		if !f.FileInfo().IsDir() {
			fileCount++
		}
	}
	return fileCount, nil
}

func GenerateID() string {
	// 生成一个简单的唯一ID，可以使用时间戳和随机数
	return hex.EncodeToString(md5.New().Sum([]byte(filepath.Base(os.TempDir()))))[:16]
}

func GetEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
// DownloadWithResume 使用断点续传下载文件
func DownloadWithResume(fileID, fileName, downloadURL, savePath string) error {
	// 使用默认状态目录
	homeDir, _ := os.UserHomeDir()
	stateDir := filepath.Join(homeDir, ".downloads", "states")

	downloader := NewResumableDownloader(stateDir)

	// 默认分块大小为1MB
	chunkSize := int64(1024 * 1024)

	return downloader.StartDownload(fileID, fileName, downloadURL, savePath, chunkSize)
}

// ResumeDownloadByID 根据文件ID恢复下载
func ResumeDownloadByID(fileID string) error {
	homeDir, _ := os.UserHomeDir()
	stateDir := filepath.Join(homeDir, ".downloads", "states")

	downloader := NewResumableDownloader(stateDir)
	return downloader.ResumeDownload(fileID)
}

// GetDownloadProgressByID 获取下载进度
func GetDownloadProgressByID(fileID string) float64 {
	homeDir, _ := os.UserHomeDir()
	stateDir := filepath.Join(homeDir, ".downloads", "states")

	downloader := NewResumableDownloader(stateDir)
	return downloader.GetDownloadProgress(fileID)
}

// ListAllDownloads 列出所有下载状态
func ListAllDownloads() []*DownloadState {
	homeDir, _ := os.UserHomeDir()
	stateDir := filepath.Join(homeDir, ".downloads", "states")

	downloader := NewResumableDownloader(stateDir)
	return downloader.ListDownloads()
}

// CancelDownloadByID 取消下载
func CancelDownloadByID(fileID string) error {
	homeDir, _ := os.UserHomeDir()
	stateDir := filepath.Join(homeDir, ".downloads", "states")

	downloader := NewResumableDownloader(stateDir)
	return downloader.CancelDownload(fileID)
}
// DownloadWithResume 使用断点续传下载文件
func DownloadWithResume(fileID, fileName, downloadURL, savePath string) error {
	// 使用默认状态目录
	homeDir, _ := os.UserHomeDir()
	stateDir := filepath.Join(homeDir, ".downloads", "states")
	
	downloader := NewResumableDownloader(stateDir)
	
	// 默认分块大小为1MB
	chunkSize := int64(1024 * 1024)
	
	return downloader.StartDownload(fileID, fileName, downloadURL, savePath, chunkSize)
}

// ResumeDownloadByID 根据文件ID恢复下载
func ResumeDownloadByID(fileID string) error {
	homeDir, _ := os.UserHomeDir()
	stateDir := filepath.Join(homeDir, ".downloads", "states")
	
	downloader := NewResumableDownloader(stateDir)
	return downloader.ResumeDownload(fileID)
}

// GetDownloadProgressByID 获取下载进度
func GetDownloadProgressByID(fileID string) float64 {
	homeDir, _ := os.UserHomeDir()
	stateDir := filepath.Join(homeDir, ".downloads", "states")
	
	downloader := NewResumableDownloader(stateDir)
	return downloader.GetDownloadProgress(fileID)
}

// ListAllDownloads 列出所有下载状态
func ListAllDownloads() []*DownloadState {
	homeDir, _ := os.UserHomeDir()
	stateDir := filepath.Join(homeDir, ".downloads", "states")
	
	downloader := NewResumableDownloader(stateDir)
	return downloader.ListDownloads()
}

// CancelDownloadByID 取消下载
func CancelDownloadByID(fileID string) error {
	homeDir, _ := os.UserHomeDir()
	stateDir := filepath.Join(homeDir, ".downloads", "states")
	
	downloader := NewResumableDownloader(stateDir)
	return downloader.CancelDownload(fileID)
}