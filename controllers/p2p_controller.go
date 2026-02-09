package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"GoFileShare/models"

	"github.com/gin-gonic/gin"
)

// P2PController P2P控制器
type P2PController struct {
	serverIP   string
	serverPort string
}

// NewP2PController 创建P2P控制器
func NewP2PController(serverIP, serverPort string) *P2PController {
	return &P2PController{
		serverIP:   serverIP,
		serverPort: serverPort,
	}
}

// ShowP2PDebugPage 显示P2P调试页面
func (ctrl *P2PController) ShowP2PDebugPage(c *gin.Context) {
	c.HTML(http.StatusOK, "p2p_debug.html", gin.H{
		"title": "P2P调试",
	})
}

// RegisterP2PKey 注册P2P密钥
func (ctrl *P2PController) RegisterP2PKey(c *gin.Context) {
	var req struct {
		Key string `json:"key"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	manager := models.GetP2PManager()
	if manager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "P2P管理器未初始化"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	info, err := manager.DiscoverP2PInfo(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "发现P2P信息失败: " + err.Error()})
		return
	}

	manager.SetP2PKey(req.Key)

	if err := manager.StartResponseListener(8080, req.Key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "启动监听器失败: " + err.Error()})
		return
	}

	// 向信令服务器注册
	if err := ctrl.registerToSignalingServer(ctx, info); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "注册到信令服务器失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "P2P密钥注册成功",
		"info":    info,
	})
}

// QueryP2PIP 查询P2P IP
func (ctrl *P2PController) QueryP2PIP(c *gin.Context) {
	targetKey := c.Query("key")
	if targetKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少目标密钥"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	holePunchInfo, err := ctrl.getHolePunchFromServer(ctx, targetKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"info":   holePunchInfo,
	})
}

// ConnectP2P 连接P2P
func (ctrl *P2PController) ConnectP2P(c *gin.Context) {
	var req struct {
		TargetKey string `json:"target_key"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	manager := models.GetP2PManager()
	if manager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "P2P管理器未初始化"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	holePunchInfo, err := ctrl.getHolePunchFromServer(ctx, req.TargetKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取打洞信息失败: " + err.Error()})
		return
	}

	info := manager.GetP2PInfo()
	if info == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "P2P信息未初始化"})
		return
	}

	if err := manager.ConnectPeer(ctx, info.Key, holePunchInfo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "连接失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "P2P连接成功",
	})
}

// TestP2PConnection 测试P2P连接
func (ctrl *P2PController) TestP2PConnection(c *gin.Context) {
	var req struct {
		TargetKey string `json:"target_key"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	manager := models.GetP2PManager()
	if manager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "P2P管理器未初始化"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	holePunchInfo, err := ctrl.getHolePunchFromServer(ctx, req.TargetKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取打洞信息失败: " + err.Error()})
		return
	}

	info := manager.GetP2PInfo()
	if info == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "P2P信息未初始化"})
		return
	}

	if err := manager.ConnectPeerTest(ctx, info.Key, holePunchInfo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "测试连接失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "P2P连接测试成功",
	})
}

// GetP2PStatus 获取P2P状态
func (ctrl *P2PController) GetP2PStatus(c *gin.Context) {
	manager := models.GetP2PManager()
	if manager == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":     "not_initialized",
			"registered": false,
		})
		return
	}

	info := manager.GetP2PInfo()
	registeredAt := manager.GetRegisteredAt()

	c.JSON(http.StatusOK, gin.H{
		"status":        "initialized",
		"registered":    registeredAt > 0,
		"registered_at": registeredAt,
		"info":          info,
	})
}

// 辅助方法：向信令服务器注册
func (ctrl *P2PController) registerToSignalingServer(ctx context.Context, info *models.P2PInfo) error {
	url := fmt.Sprintf("http://%s:%s/register", ctrl.serverIP, ctrl.serverPort)
	
	data := map[string]string{
		"key":  info.Key,
		"ip":   info.OutIP,
		"port": info.OutPort,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(nil)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("注册失败: %s", string(body))
	}

	_ = jsonData // 避免未使用变量警告
	return nil
}

// 辅助方法：从信令服务器获取打洞信息
func (ctrl *P2PController) getHolePunchFromServer(ctx context.Context, targetKey string) (*models.HolePunchInfo, error) {
	url := fmt.Sprintf("http://%s:%s/query?key=%s", ctrl.serverIP, ctrl.serverPort, targetKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("查询失败: %s", string(body))
	}

	var result struct {
		IP   string `json:"ip"`
		Port string `json:"port"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &models.HolePunchInfo{
		ExternalIP:   result.IP,
		ExternalPort: result.Port,
		HasRequest:   true,
	}, nil
}
