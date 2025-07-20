package controllers

import (
	"GoFileShare/config"
	"fmt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"

	"GoFileShare/models"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
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

	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "获取文件失败"})
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "关闭文件失败"})
			return
		}
	}(file)

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

	if len(fileNode) != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件系统错误，请检查文件ID"})
		return
	}

	// 构建文件保存路径
	fileName := header.Filename
	uploadPath := filepath.Join(fileNode[0].Path, fileName)

	// 保存文件到磁盘
	err = c.SaveUploadedFile(header, uploadPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}

	// 添加文件节点到数据库
	err = models.AddFileNode(uploadPath, fileName, false, fileNode[0].ID.Hex(), authLevel.(*int))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加文件节点失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "filename": fileName})
}
