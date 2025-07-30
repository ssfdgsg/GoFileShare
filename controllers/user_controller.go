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

	name := c.Param("NAME")
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
	authLevel := session.Get("auth_level")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未登录",
		})
		return
	}

	nodeID := c.Param("id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "缺少文件节点ID",
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

	fileNodes, err := models.SearchFileNodeByID(objID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"error": err.Error(),
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

// InitDownloadTask 初始化下载任务
func InitDownloadTask(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	authLevel := session.Get("auth_level")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
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

	downloadTask, err := config.AuthCheck(authLevel.(int), fileNode)
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
	authLevel := session.Get("auth_level")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
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

	downloadTask, err := config.AuthCheck(authLevel.(int), fileNode)
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
	parentID := c.Param("NAME")
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
