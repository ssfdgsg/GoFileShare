package controllers

import (
	"GoFileShare/config"
	"GoFileShare/models"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ShowHomePage 显示主页
func ShowHomePage(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	loginTime := session.Get("login_time")

	fmt.Printf("访问主页，用户: %v, 登录时间: %v\n", username, loginTime)

	c.HTML(http.StatusOK, "home.html", gin.H{
		"title":       "主页",
		"username":    username,
		"currentTime": time.Now().Format("2006-01-02 15:04:05"),
		"loginTime":   loginTime,
	})
}

// GetUserInfo 获取用户信息API
func GetUserInfo(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")

	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未登录",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"username": username,
		"status":   "success",
	})
}

// GetUserByName 根据用户名获取用户详细信息
func GetUserByName(c *gin.Context) {
	username := c.Param("name")

	user, err := models.GetUserByName(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
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

// ListFilesByName 根据名称搜索文件
func ListFilesByName(c *gin.Context, name string) {
	session := sessions.Default(c)
	username := session.Get("user")
	authLevel := session.Get("auth_level")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未登录",
		})
		return
	}

	fileNodes, err := models.SearchFileNodeByName(name)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"error": err.Error(),
		})
	}
	checkedFileNodes, err := config.AuthCheck(authLevel.(int), fileNodes)
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

// ListFileDirByName 根据文件名列出文件目录
func ListFileDirByName(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	authLevel := session.Get("authLevel")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未登录",
		})
		return
	}

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
	checkedFileNodes, err := config.AuthCheck(authLevel.(int), fileNodes)
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
	session := sessions.Default(c)
	username := session.Get("user")
	authLevel := session.Get("authLevel") // 统一使用 authLevel
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未登录",
		})
		return
	}

	// 安全地获取权限等级
	auth, ok := authLevel.(int)
	if !ok {
		// 如果获取失败，默认为0（普通用户）
		auth = 0
	}

	nodeID := c.Param("id")
	if nodeID == "" || nodeID == "root" {
		// 如果是根目录，获取所有父节点为nil的文件
		fileNodes, err := models.SearchFileNodeByParentID(primitive.NilObjectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		checkedFileNodes, err := config.AuthCheck(auth, fileNodes)
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

	objID, err := primitive.ObjectIDFromHex(nodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的文件节点ID",
		})
		return
	}

	// 根据父节点ID获取子文件和文件夹
	fileNodes, err := models.SearchFileNodeByParentID(objID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	checkedFileNodes, err := config.AuthCheck(auth, fileNodes)
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
	session := sessions.Default(c)
	username := session.Get("user")
	authLevel := session.Get("authLevel") // 统一使用 authLevel
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	// 安全地获取权限等级
	auth, ok := authLevel.(int)
	if !ok {
		// 如果获取失败，默认为0（普通用户）
		auth = 0
	}

	nodeID := c.Param("id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件节点ID"})
		return
	}

	objID, err := primitive.ObjectIDFromHex(nodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文件节点ID"})
		return
	}

	fileNode, err := models.SearchFileNodeByID(objID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
func StartDownload(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	authLevel := session.Get("authLevel") // 统一使用 authLevel
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	// 安全地获取权限等级
	auth, ok := authLevel.(int)
	if !ok {
		// 如果获取失败，默认为0（普通用户）
		auth = 0
	}

	nodeID := c.Param("id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件节点ID"})
		return
	}

	objID, err := primitive.ObjectIDFromHex(nodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文件节点ID"})
		return
	}

	fileNode, err := models.SearchFileNodeByID(objID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	downloadTask, err := config.AuthCheck(auth, fileNode)
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
	session := sessions.Default(c)
	username := session.Get("user")

	// 统一使用 authLevel 作为键名（与登录函数保持一致）
	authLevel := session.Get("authLevel")

	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	auth, ok := authLevel.(int)
	if !ok {
		// 权限默认为0（普通用户）
		auth = 0
	}

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
	err = models.AddFileNode(filePath, fileName, false, parentID, auth)
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

// UpdateDir创建文件夹
func UpdateDir(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	authLevel := session.Get("authLevel")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

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

	auth, ok := authLevel.(int)
	if !ok {
		auth = 0
	}

	err := models.AddFileNode("", addDirName, true, parentID, auth)
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
	session := sessions.Default(c)
	username := session.Get("user")
	authLevel := session.Get("authLevel")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	// 安全地获取权限等级
	auth, ok := authLevel.(int)
	if !ok {
		auth = 0
	}

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
func DeleteFile(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	authLevel := session.Get("authLevel")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	// 安全地获取权限等级
	auth, ok := authLevel.(int)
	if !ok {
		auth = 0
	}

	nodeID := c.Param("id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件节点ID"})
		return
	}

	objID, err := primitive.ObjectIDFromHex(nodeID)
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
	checkedFileNodes, err := config.AuthCheck(auth, []config.FileNode{fileNode})
	if err != nil || len(checkedFileNodes) == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足，无法删除此文件"})
		return
	}

	// 删除文件节点和所有子节点（如果是文件夹）
	err = models.DeleteFileNodeWithChildren(nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除文件节点失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "删除成功",
		"name":    fileNode.Name,
	})
}
