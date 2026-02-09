package handlers

import (
	"context"
	"net/http"
	"time"

	"GoFileShare/internal/service"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type P2PHandler struct {
	p2pService *service.P2PService
}

func NewP2PHandler(p2pService *service.P2PService) *P2PHandler {
	return &P2PHandler{p2pService: p2pService}
}

// RegisterP2PKey 注册P2P密钥
func (h *P2PHandler) RegisterP2PKey(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	key := c.PostForm("key")

	info, err := h.p2pService.Init(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "初始化P2P服务失败",
			"message": err.Error(),
		})
		return
	}

	if info == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "P2P服务初始化失败",
			"message": "服务对象为空",
		})
		return
	}

	if key != "" {
		h.p2pService.OverrideKey(key)
	}

	err = h.p2pService.Register(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "注册失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "注册成功",
		"key":     info.Key,
		"ip":      info.OutIP,
		"port":    info.OutPort,
	})
}

// QueryP2PIP 查询P2P密钥对应的客户端信息
func (h *P2PHandler) QueryP2PIP(c *gin.Context) {
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

	holePunchInfo, err := h.p2pService.GetHolePunch(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "查询失败", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": holePunchInfo})
}

// ShowP2PDebugPage 显示P2P调试页面
func (h *P2PHandler) ShowP2PDebugPage(c *gin.Context) {
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

// ConnectP2P 建立P2P连接
func (h *P2PHandler) ConnectP2P(c *gin.Context) {
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

	if h.p2pService.Info() == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "P2P服务未初始化"})
		return
	}

	holePunchInfo, err := h.p2pService.GetHolePunch(c.Request.Context(), targetKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "获取打洞信息失败",
			"message": err.Error(),
		})
		return
	}

	if !holePunchInfo.HasRequest {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "未找到目标客户端",
			"message": "目标客户端可能不在线或密钥错误",
		})
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = h.p2pService.ConnectPeer(ctx, holePunchInfo)
	}()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "正在建立P2P连接",
		"data": gin.H{
			"target_ip":   holePunchInfo.ExternalIP,
			"target_port": holePunchInfo.ExternalPort,
			"requester":   holePunchInfo.RequesterKey,
		},
	})
}

// TestP2PConnection 测试P2P连接
func (h *P2PHandler) TestP2PConnection(c *gin.Context) {
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

	if h.p2pService.Info() == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "P2P服务未初始化"})
		return
	}

	holePunchInfo, err := h.p2pService.GetHolePunch(c.Request.Context(), targetKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "获取打洞信息失败",
			"message": err.Error(),
		})
		return
	}

	if !holePunchInfo.HasRequest {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "未找到目标客户端",
			"message": "目标客户端可能不在线或密钥错误",
		})
		return
	}

	err = h.p2pService.ConnectPeerTest(c.Request.Context(), holePunchInfo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "连接测试失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "P2P连接测试成功",
	})
}

// GetP2PStatus 获取P2P状态
func (h *P2PHandler) GetP2PStatus(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	info := h.p2pService.Info()
	if info == nil {
		c.JSON(http.StatusOK, gin.H{
			"status": "not_initialized",
			"data": gin.H{
				"initialized": false,
				"message":     "P2P服务未初始化",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"initialized":    true,
			"key":            info.Key,
			"external_ip":    info.OutIP,
			"external_port":  info.OutPort,
			"registered_at":  h.p2pService.RegisteredAtUnix(),
		},
	})
}
