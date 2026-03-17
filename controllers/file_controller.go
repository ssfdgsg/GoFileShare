package controllers

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"GoFileShare/config"
	"GoFileShare/models"
	"GoFileShare/utils"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// FileController 文件控制器
type FileController struct {
	rootPath string
}

// NewFileController 创建文件控制器
func NewFileController(rootPath string) *FileController {
	return &FileController{rootPath: rootPath}
}

// ShowHomePage 显示主页
func (ctrl *FileController) ShowHomePage(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	loginTime := session.Get("login_time")

	c.HTML(http.StatusOK, "home.html", gin.H{
		"title":       "主页",
		"username":    username,
		"currentTime": time.Now().Format("2006-01-02 15:04:05"),
		"loginTime":   loginTime,
	})
}

// GetUserInfo 获取用户信息API
func (ctrl *FileController) GetUserInfo(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")

	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"username": username,
		"status":   "success",
	})
}

// GetUserByName 根据用户名获取用户详细信息
func (ctrl *FileController) GetUserByName(c *gin.Context) {
	username := c.Param("name")

	user, err := models.GetUserByName(c.Request.Context(), username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if user == nil {
		c.JSON(http.StatusOK, gin.H{
			"user":   username,
			"status": "no value",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": username,
		"data": user,
	})
}

// ListFileDirByName 根据文件名列出文件目录
func (ctrl *FileController) ListFileDirByName(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	authLevel := session.Get("authLevel")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件名"})
		return
	}

	fileNodes, err := models.SearchFileNodeByName(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error() + "搜索文件名失败！"})
		return
	}
	
	checkedFileNodes, err := config.AuthCheck(authLevel.(int), fileNodes)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"files": checkedFileNodes})
}

// ListFileDirByID 根据文件节点ID列出文件目录
func (ctrl *FileController) ListFileDirByID(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	authLevel := session.Get("authLevel")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	auth, ok := authLevel.(int)
	if !ok {
		auth = 0
	}

	nodeID := c.Param("id")
	if nodeID == "" || nodeID == "root" {
		fileNodes, err := models.SearchFileNodeByParentID(c.Request.Context(), "root")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		checkedFileNodes, err := config.AuthCheck(auth, fileNodes)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"files": checkedFileNodes})
		return
	}

	fileNodes, err := models.SearchFileNodeByParentID(c.Request.Context(), nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	checkedFileNodes, err := config.AuthCheck(auth, fileNodes)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"files": checkedFileNodes})
}

// InitDownloadTask 初始化下载任务
func (ctrl *FileController) InitDownloadTask(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	authLevel := session.Get("authLevel")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	auth, ok := authLevel.(int)
	if !ok {
		auth = 0
	}

	nodeID := c.Param("id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件节点ID"})
		return
	}

	fileNode, err := models.SearchFileNodeByID(c.Request.Context(), nodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文件节点ID"})
		return
	}

	downloadTask, err := config.AuthCheck(auth, fileNode)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"task": downloadTask})
}

// StartDownload 提供下载接口
func (ctrl *FileController) StartDownload(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	authLevel := session.Get("authLevel")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	auth, ok := authLevel.(int)
	if !ok {
		auth = 0
	}

	nodeID := c.Param("id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件节点ID"})
		return
	}

	fileNode, err := models.SearchFileNodeByID(c.Request.Context(), nodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文件节点ID"})
		return
	}

	downloadTask, err := config.AuthCheck(auth, fileNode)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if len(downloadTask) > 0 {
		c.File(downloadTask[0].Storage.SystemFilePath)
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
	}
}

// StartUpload 提供上传接口
func (ctrl *FileController) StartUpload(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	authLevel := session.Get("authLevel")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	auth, ok := authLevel.(int)
	if !ok {
		auth = 0
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "获取文件失败"})
		return
	}
	defer file.Close()

	fileName := header.Filename
	filePath := filepath.Join(ctrl.rootPath, "FileStore", fileName)

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建目录失败"})
		return
	}

	parentID := c.Param("id")
	if parentID == "" || parentID == "undefined" || parentID == "null" {
		parentID = "root"
	}

	if err := c.SaveUploadedFile(header, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}

	// 使用并发 MD5 校验（大文件时更快）
	md5Hash, err := utils.MD5CheckConcurrent(filePath, 4)
	if err != nil {
		// 如果并发校验失败，降级到单线程
		md5Hash = utils.MD5Check(filePath)
	}

	if err := models.AddFileNode(c.Request.Context(), filePath, fileName, false, parentID, auth); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加文件节点失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"filename": fileName,
		"message":  "文件上传成功",
		"md5":      md5Hash,
	})
}

// UpdateDir 创建文件夹
func (ctrl *FileController) UpdateDir(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	authLevel := session.Get("authLevel")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	parentID := c.Param("id")
	if parentID == "" || parentID == "undefined" || parentID == "null" {
		parentID = "root"
	}

	addDirName := c.PostForm("addDirName")
	if addDirName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件夹名称"})
		return
	}

	auth, ok := authLevel.(int)
	if !ok {
		auth = 0
	}

	if err := models.AddFileNode(c.Request.Context(), "", addDirName, true, parentID, auth); err != nil {
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
func (ctrl *FileController) SearchFiles(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	authLevel := session.Get("authLevel")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	auth, ok := authLevel.(int)
	if !ok {
		auth = 0
	}

	searchTerm := c.Query("q")
	if searchTerm == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少搜索关键词"})
		return
	}

	fileNodes, err := models.SearchFileNodeByNamePattern(c.Request.Context(), searchTerm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "搜索失败: " + err.Error()})
		return
	}

	checkedFileNodes, err := config.AuthCheck(auth, fileNodes)
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
func (ctrl *FileController) DeleteFile(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	authLevel := session.Get("authLevel")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	auth, ok := authLevel.(int)
	if !ok {
		auth = 0
	}

	nodeID := c.Param("id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件节点ID"})
		return
	}

	fileNodes, err := models.SearchFileNodeByID(c.Request.Context(), nodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文件节点ID"})
		return
	}

	if len(fileNodes) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}

	fileNode := fileNodes[0]

	checkedFileNodes, err := config.AuthCheck(auth, []models.FileNode{fileNode})
	if err != nil || len(checkedFileNodes) == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足，无法删除此文件"})
		return
	}

	if err := models.DeleteFileNodeWithChildren(c.Request.Context(), nodeID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除文件节点失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "删除成功",
		"name":    fileNode.Name,
	})
}
