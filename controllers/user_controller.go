package controllers

import (
	"GoFileShare/config"
	"GoFileShare/models"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// ShowHomePage 显示主页
func ShowHomePage(c *gin.Context) {
	c.HTML(http.StatusOK, "home.html", gin.H{
		"title":       "文件共享",
		"currentTime": time.Now().Format("2006-01-02 15:04:05"),
	})
}

// ListFileDirByName 根据文件名列出文件目录
func ListFileDirByName(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "缺少文件名",
		})
		return
	}

	fileNodes, err := models.SearchFileNodeByName(name)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"error": err.Error() + "搜索文件名失败！",
		})
		return
	}

	// 默认最高权限
	authLevel := 100
	checkedFileNodes, err := config.AuthCheck(authLevel, fileNodes)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"files": checkedFileNodes,
	})
}

// ListFileDirByID 根据文件节点ID列出文件目录
func ListFileDirByID(c *gin.Context) {
	nodeID := c.Param("id")

	// 默认最高权限
	authLevel := 100

	if nodeID == "" || nodeID == "root" {
		// 如果是根目录，获取所有父节点为null的文件
		fileNodes, err := models.SearchFileNodeByParentID(nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		checkedFileNodes, err := config.AuthCheck(authLevel, fileNodes)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"files": checkedFileNodes,
		})
		return
	}

	objID, err := strconv.ParseInt(nodeID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的文件节点ID",
		})
		return
	}

	// 根据父节点ID获取子文件和文件夹
	fileNodes, err := models.SearchFileNodeByParentID(&objID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	checkedFileNodes, err := config.AuthCheck(authLevel, fileNodes)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"files": checkedFileNodes,
	})
}

// InitDownloadTask 初始化下载任务
func InitDownloadTask(c *gin.Context) {
	// 默认最高权限
	authLevel := 100

	nodeID := c.Param("id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件节点ID"})
		return
	}

	objID, err := strconv.ParseInt(nodeID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文件节点ID"})
		return
	}

	fileNode, err := models.SearchFileNodeByID(objID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	downloadTask, err := config.AuthCheck(authLevel, fileNode)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"task": downloadTask})
}

// StartDownload 提供下载接口
func StartDownload(c *gin.Context) {
	// 默认最高权限
	authLevel := 100

	nodeID := c.Param("id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件节点ID"})
		return
	}

	objID, err := strconv.ParseInt(nodeID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文件节点ID"})
		return
	}

	fileNode, err := models.SearchFileNodeByID(objID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	downloadTask, err := config.AuthCheck(authLevel, fileNode)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if len(downloadTask) > 0 {
		// 只下载第一个文件
		c.File(downloadTask[0].Storage.SystemFilePath)
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
	}
}

// StartUpload 提供上传接口
func StartUpload(c *gin.Context) {
	// 默认最高权限
	authLevel := 100

	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "获取文件失败"})
		return
	}
	defer file.Close()

	fileName := header.Filename
	filePath := filepath.Join(config.RootPath, "FileStore", fileName)

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建目录失败"})
		return
	}

	// 从URL参数获取父目录ID
	parentID := c.Param("id")
	if parentID == "" || parentID == "undefined" || parentID == "null" {
		parentID = "root"
	}

	// 保存上传的文件
	if err := c.SaveUploadedFile(header, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}

	// 文件保存成功后添加节点记录
	err = models.AddFileNode(filePath, fileName, false, parentID, authLevel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加文件节点失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"filename": fileName,
		"message":  "文件上传成功",
	})
}

// UpdateDir 创建文件夹
func UpdateDir(c *gin.Context) {
	// 默认最高权限
	authLevel := 100

	// 从URL参数获取父目录ID
	parentID := c.Param("id")
	if parentID == "" || parentID == "undefined" || parentID == "null" {
		parentID = "root"
	}

	addDirName := c.PostForm("addDirName")
	if addDirName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件夹名称"})
		return
	}

	err := models.AddFileNode("", addDirName, true, parentID, authLevel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建文件夹失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "文件夹创建成功",
		"name":    addDirName,
	})
}

// SearchFiles 搜索文件
func SearchFiles(c *gin.Context) {
	// 默认最高权限
	authLevel := 100

	// 获取搜索关键词
	searchTerm := c.Query("q")
	if searchTerm == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少搜索关键词"})
		return
	}

	// 调用模型层的搜索函数
	fileNodes, err := models.SearchFileNodeByNamePattern(searchTerm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "搜索失败: " + err.Error()})
		return
	}

	// 权限检查
	checkedFileNodes, err := config.AuthCheck(authLevel, fileNodes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "权限检查失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"files": checkedFileNodes,
		"count": len(checkedFileNodes),
		"query": searchTerm,
	})
}

// DeleteFile 删除文件或文件夹
func DeleteFile(c *gin.Context) {
	// 默认最高权限
	authLevel := 100

	nodeID := c.Param("id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件节点ID"})
		return
	}

	objID, err := strconv.ParseInt(nodeID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文件节点ID"})
		return
	}

	// 首先查找文件节点
	fileNodes, err := models.SearchFileNodeByID(objID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查找文件失败: " + err.Error()})
		return
	}

	if len(fileNodes) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}

	fileNode := fileNodes[0]

	// 权限检查
	if fileNode.EffectiveAuthLevel > authLevel {
		c.JSON(http.StatusForbidden, gin.H{"error": "没有权限删除此文件"})
		return
	}

	// 删除节点及其所有子节点
	if err := models.DeleteFileNodeWithChildren(nodeID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "删除成功",
		"name":    fileNode.Name,
	})
}
