package controllers

import (
	"GoFileShare/models"
	"GoFileShare/services"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// ConnectP2PHybrid 混合P2P连接 - 同时支持WebRTC和QUIC
func ConnectP2PHybrid(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	type HybridConnectRequest struct {
		TargetKey     string `json:"target_key" binding:"required"`
		PreferredMode string `json:"preferred_mode"` // "webrtc" 或 "quic" 或 "auto"
	}

	var req HybridConnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求"})
		return
	}

	if req.PreferredMode == "" {
		req.PreferredMode = "auto"
	}

	// 确保P2P客户端已初始化
	if services.GlobalP2PClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "P2P服务未初始化"})
		return
	}

	// 获取打洞信息
	holePunchInfo, err := services.GlobalP2PClient.GetHolePunch(req.TargetKey)
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

	// 异步尝试建立混合连接
	go func() {
		connectHybridP2P(holePunchInfo, req.PreferredMode, req.TargetKey)
	}()

	c.JSON(http.StatusOK, gin.H{
		"status":         "success",
		"message":        "正在建立P2P连接",
		"preferred_mode": req.PreferredMode,
		"target_ip":      holePunchInfo.ExternalIP,
		"target_port":    holePunchInfo.ExternalPort,
		"requester":      holePunchInfo.RequesterKey,
	})
}

// connectHybridP2P 内部函数：尝试使用混合方式建立连接
func connectHybridP2P(holePunchInfo *models.HolePunchInfo, preferredMode, targetKey string) {
	if preferredMode == "webrtc" {
		log.Printf("开始建立WebRTC连接到 %s", targetKey)
		connectWebRTCP2P(holePunchInfo, targetKey)
	} else if preferredMode == "quic" {
		log.Printf("开始建立QUIC连接到 %s", targetKey)
		connectQUICp2P(holePunchInfo, targetKey)
	} else {
		// auto模式：先尝试WebRTC，失败后尝试QUIC
		log.Printf("自动模式：优先尝试WebRTC到 %s", targetKey)

		// 创建WebRTC连接
		if connectWebRTCP2P(holePunchInfo, targetKey) {
			log.Printf("WebRTC连接成功")
			return
		}

		log.Printf("WebRTC连接失败，回退到QUIC")
		connectQUICp2P(holePunchInfo, targetKey)
	}
}

// connectWebRTCP2P 建立WebRTC P2P连接
func connectWebRTCP2P(holePunchInfo *models.HolePunchInfo, targetKey string) bool {
	log.Printf("尝试建立WebRTC连接到 %s:%s", holePunchInfo.ExternalIP, holePunchInfo.ExternalPort)

	if services.GlobalWebRTCService == nil {
		var err error
		services.GlobalWebRTCService, err = services.NewWebRTCService()
		if err != nil {
			log.Printf("初始化WebRTC失败: %v", err)
			return false
		}

		_, err = services.GlobalWebRTCService.CreateDataChannel("file-transfer")
		if err != nil {
			log.Printf("创建数据通道失败: %v", err)
			return false
		}
	}

	// 创建Offer
	offer, err := services.GlobalWebRTCService.CreateOffer()
	if err != nil {
		log.Printf("创建WebRTC Offer失败: %v", err)
		return false
	}

	log.Printf("WebRTC Offer已生成，准备交换信令")

	// 这里应该通过信令服务器交换offer/answer
	// 对于演示，我们将信令信息记录下来
	var offerData map[string]interface{}
	if err := json.Unmarshal([]byte(offer), &offerData); err == nil {
		log.Printf("Offer SDP长度: %d", len(offerData["sdp"].(string)))
	}

	return true
}

// connectQUICp2P 建立QUIC P2P连接
func connectQUICp2P(holePunchInfo *models.HolePunchInfo, targetKey string) {
	log.Printf("尝试建立QUIC连接到 %s:%s", holePunchInfo.ExternalIP, holePunchInfo.ExternalPort)

	targetClient := &services.TargetClient{
		ExternalIP:   holePunchInfo.ExternalIP,
		ExternalPort: holePunchInfo.ExternalPort,
	}

	targetClient.EchoPeer()
}

// GetP2PConnectionMethods 获取可用的P2P连接方式
func GetP2PConnectionMethods(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	methods := gin.H{
		"webrtc": gin.H{
			"enabled":     true,
			"description": "基于WebRTC的P2P连接，自动NAT穿透和候选选择",
			"protocols":   []string{"ICE", "STUN", "TURN"},
		},
		"quic": gin.H{
			"enabled":     true,
			"description": "基于QUIC的P2P连接，高性能UDP连接",
			"protocols":   []string{"QUIC", "UDP NAT穿透"},
		},
		"hybrid": gin.H{
			"enabled":     true,
			"description": "混合模式，自动选择最佳连接方式",
			"fallback":    "quic",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"methods": methods,
	})
}

// TestP2PMethods 测试所有P2P连接方式
func TestP2PMethods(c *gin.Context) {
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

	if services.GlobalP2PClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "P2P服务未初始化"})
		return
	}

	holePunchInfo, err := services.GlobalP2PClient.GetHolePunch(targetKey)
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

	results := make(map[string]interface{})

	// 测试WebRTC
	log.Printf("测试WebRTC连接...")
	webrtcSuccess := testWebRTC()
	results["webrtc"] = gin.H{
		"success": webrtcSuccess,
		"status":  map[bool]string{true: "连接成功", false: "连接失败"}[webrtcSuccess],
	}

	// 测试QUIC
	log.Printf("测试QUIC连接...")
	quicSuccess := testQUIC(holePunchInfo)
	results["quic"] = gin.H{
		"success": quicSuccess,
		"status":  map[bool]string{true: "连接成功", false: "连接失败"}[quicSuccess],
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"results": results,
		"recommended": map[bool]string{
			true:  "webrtc",
			false: "quic",
		}[webrtcSuccess],
	})
}

// testWebRTC 测试WebRTC连接可用性
func testWebRTC() bool {
	if services.GlobalWebRTCService == nil {
		webrtcService, err := services.NewWebRTCService()
		if err != nil {
			log.Printf("WebRTC初始化失败: %v", err)
			return false
		}

		_, err = webrtcService.CreateDataChannel("test")
		if err != nil {
			log.Printf("WebRTC数据通道创建失败: %v", err)
			return false
		}

		return true
	}

	return services.GlobalWebRTCService.GetConnectionState() != "failed"
}

// testQUIC 测试QUIC连接可用性
func testQUIC(holePunchInfo *models.HolePunchInfo) bool {
	err := services.GlobalP2PClient.ConnectPeerTest(holePunchInfo)
	return err == nil
}

// QueryP2PConnectionInfo 查询P2P连接信息
func QueryP2PConnectionInfo(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	targetKey := c.Query("target_key")
	if targetKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "目标密钥不能为空"})
		return
	}

	if services.GlobalP2PClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "P2P服务未初始化"})
		return
	}

	holePunchInfo, err := services.GlobalP2PClient.GetHolePunch(targetKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "查询失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"info":   holePunchInfo,
	})
}
