package controllers

import (
	"GoFileShare/services"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"net/http"
)

// GetP2PStatus 获取P2P连接状态
func GetP2PStatus(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	client := services.GetGlobalEnhancedP2PClient()
	if client == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":    "disconnected",
			"message":   "P2P客户端未初始化",
			"connected": false,
		})
		return
	}

	// 获取详细的客户端信息
	clientInfo := client.GetClientInfo()
	connections := client.GetP2PConnections()

	c.JSON(http.StatusOK, gin.H{
		"status":      "connected",
		"message":     "P2P客户端已连接",
		"connected":   true,
		"client_info": clientInfo,
		"connections": connections,
		"peer_count":  len(connections),
	})
}

// RegisterP2PKey 注册P2P密钥
func RegisterP2PKey(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	key := c.PostForm("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密钥不能为空"})
		return
	}

	client := services.GetGlobalEnhancedP2PClient()
	if client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "P2P客户端未初始化"})
		return
	}

	err := client.Register()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "注册失败",
			"message": err.Error(),
		})
		return
	}

	// 获取注册后的客户端信息
	clientInfo := client.GetClientInfo()

	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"message":     "注册成功",
		"client_info": clientInfo,
	})
}

// QueryP2PIP 查询P2P密钥对应的客户端信息
func QueryP2PIP(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	key := c.Query("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密钥不能为空"})
		return
	}

	client := services.GetGlobalEnhancedP2PClient()
	if client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "P2P客户端未初始化"})
		return
	}

	// 先尝试连接到目标节点来获取信息
	err := client.ConnectToPeer(key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "查询失败",
			"message": err.Error(),
		})
		return
	}

	// 获取连接信息
	connections := client.GetP2PConnections()
	if targetConn, exists := connections[key]; exists {
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"key":    key,
			"target_info": map[string]interface{}{
				"external_ip":   targetConn.RemoteExternalIP,
				"external_port": targetConn.RemoteExternalPort,
				"local_ip":      targetConn.RemoteLocalIP,
				"local_port":    targetConn.RemoteLocalPort,
				"is_connected":  targetConn.IsConnected,
			},
		})
		return
	}

	c.JSON(http.StatusNotFound, gin.H{
		"error":   "查询失败",
		"message": "未找到目标客户端信息",
	})
}

// ConnectP2PPeer 连接到P2P对等节点
func ConnectP2PPeer(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	targetKey := c.PostForm("target_key")
	if targetKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "目标密钥不能为空"})
		return
	}

	client := services.GetGlobalEnhancedP2PClient()
	if client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "P2P客户端未初始化"})
		return
	}

	err := client.ConnectToPeer(targetKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "连接失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"message":    "P2P连接建立成功",
		"target_key": targetKey,
	})
}

// SendP2PMessage 发送P2P消息
func SendP2PMessage(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	targetKey := c.PostForm("target_key")
	message := c.PostForm("message")

	if targetKey == "" || message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "目标密钥和消息不能为空"})
		return
	}

	client := services.GetGlobalEnhancedP2PClient()
	if client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "P2P客户端未初始化"})
		return
	}

	err := client.SendP2PMessage(targetKey, message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "发送消息失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"message":    "消息发送成功",
		"target_key": targetKey,
		"content":    message,
	})
}

// GetP2PConnections 获取P2P连接列表
func GetP2PConnections(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	client := services.GetGlobalEnhancedP2PClient()
	if client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "P2P客户端未初始化"})
		return
	}

	connections := client.GetP2PConnections()

	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"connections": connections,
		"count":       len(connections),
	})
}

// ShowP2PDebugPage 显示P2P调试页面
func ShowP2PDebugPage(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.Redirect(http.StatusFound, "/login.html")
		return
	}

	c.HTML(http.StatusOK, "p2p_debug.html", gin.H{
		"title":    "P2P调试界面",
		"username": username,
	})
}
