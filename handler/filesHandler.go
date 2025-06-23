package handler

import (
	"GoFileShare/services"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"path/filepath"
)

type FileHandler struct {
	downloadService *services.DownloadService
	uploadService   *services.UploadService
	tempDir         string // 临时文件目录
	targetDir       string
}

func NewFileHandler(downloadService *services.DownloadService, uploadService *services.UploadService, tempDir string, targetDir string) *FileHandler {
	os.MkdirAll(tempDir, 0755)
	os.MkdirAll(targetDir, 0755)

	metaDir := filepath.Join(tempDir, "metadata")
	os.MkdirAll(metaDir, 0755)
	return &FileHandler{
		downloadService: downloadService,
		uploadService:   uploadService,
		tempDir:         tempDir,
		targetDir:       targetDir,
	}
}

func (h *FileHandler) RegisterRoutes(r *gin.Engine) {
	// 下载API
	r.POST("/api/download", h.InitDownload)
	r.GET("/api/download/:id/status", h.GetDownloadStatus)
	r.GET("/api/download/:id/file", h.DownloadFile)

	// 上传API
	r.POST("/api/upload/init", h.InitUpload)
	r.POST("/api/upload/chunk", h.UploadChunk)
	r.POST("/api/upload/complete", h.CompleteUpload)
	r.GET("/api/upload/:id/status", h.GetUploadStatus)
}

func (h *FileHandler) InitDownload(c *gin.Context) {
	var req struct {
		URL       string `json:"url" binding:"required"`
		FileName  string `json:"fileName"`
		ChunkSize int64  `json:"chunkSize"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ChunkSize <= 0 {
		req.ChunkSize = 1024 * 1024 // 默认1MB
	}

	fileSize, err := h.downloadService.GetFileSize(req.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fail to get file size"})
		return
	}
	WorkerTheardCount := 4 // 假设使用4个工作线程
	if fileSize < req.ChunkSize {
		req.ChunkSize = fileSize // 如果文件小于分块大小，则使用文件大小
		WorkerTheardCount = 1    // 如果文件小于分块大小，则只使用一个工作线程
	}
	if req.FileName == "" {
		req.FileName = filepath.Base(req.URL)
	}

	filePath := filepath.Join(h.tempDir, req.FileName)
	downloadTask := services.NewDownloadTask(req.URL, filePath, req.ChunkSize)

	h.downloadService.AddTask(downloadTask)
}
