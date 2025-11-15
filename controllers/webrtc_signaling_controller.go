package controllers

import (
	"GoFileShare/services"
	"encoding/json"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// RegisterSignalingClient 注册信令客户端
func RegisterSignalingClient(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	services.InitSignalingServer()

	type RegisterRequest struct {
		ClientID     string `json:"client_id" binding:"required"`
		ExternalIP   string `json:"external_ip" binding:"required"`
		ExternalPort string `json:"external_port" binding:"required"`
	}

	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求"})
		return
	}

	err := services.GlobalSignalingServer.RegisterClient(req.ClientID, req.ExternalIP, req.ExternalPort)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "注册失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "客户端已注册",
	})
}

// GetSignalingClientInfo 获取信令客户端信息
func GetSignalingClientInfo(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	services.InitSignalingServer()

	clientID := c.Query("client_id")
	if clientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "client_id参数缺失"})
		return
	}

	info, err := services.GlobalSignalingServer.GetClientInfo(clientID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "客户端未找到",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"client_id":     info.ClientID,
			"external_ip":   info.ExternalIP,
			"external_port": info.ExternalPort,
			"connected":     info.Connected,
		},
	})
}

// ExchangeWebRTCOffer 交换WebRTC Offer
func ExchangeWebRTCOffer(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	services.InitSignalingServer()

	type OfferExchangeRequest struct {
		FromClientID string `json:"from_client_id" binding:"required"`
		ToClientID   string `json:"to_client_id" binding:"required"`
		SDP          string `json:"sdp" binding:"required"`
	}

	var req OfferExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求"})
		return
	}

	// 创建消息
	payload, _ := json.Marshal(services.OfferPayload{SDP: req.SDP})
	msg := services.SignalingMessage{
		Type:    "offer",
		From:    req.FromClientID,
		To:      req.ToClientID,
		Payload: payload,
	}

	err := services.GlobalSignalingServer.SendMessage(msg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "发送Offer失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Offer已发送",
	})
}

// ExchangeWebRTCAnswer 交换WebRTC Answer
func ExchangeWebRTCAnswer(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	services.InitSignalingServer()

	type AnswerExchangeRequest struct {
		FromClientID string `json:"from_client_id" binding:"required"`
		ToClientID   string `json:"to_client_id" binding:"required"`
		SDP          string `json:"sdp" binding:"required"`
	}

	var req AnswerExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求"})
		return
	}

	// 创建消息
	payload, _ := json.Marshal(services.AnswerPayload{SDP: req.SDP})
	msg := services.SignalingMessage{
		Type:    "answer",
		From:    req.FromClientID,
		To:      req.ToClientID,
		Payload: payload,
	}

	err := services.GlobalSignalingServer.SendMessage(msg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "发送Answer失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Answer已发送",
	})
}

// ExchangeICECandidate 交换ICE候选
func ExchangeICECandidate(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	services.InitSignalingServer()

	type CandidateExchangeRequest struct {
		FromClientID  string  `json:"from_client_id" binding:"required"`
		ToClientID    string  `json:"to_client_id" binding:"required"`
		Candidate     string  `json:"candidate" binding:"required"`
		SDPMLineIndex *uint16 `json:"sdp_mline_index"`
		SDPMid        *string `json:"sdp_mid"`
	}

	var req CandidateExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求"})
		return
	}

	// 创建消息
	payload, _ := json.Marshal(services.CandidatePayload{
		Candidate:     req.Candidate,
		SDPMLineIndex: req.SDPMLineIndex,
		SDPMid:        req.SDPMid,
	})
	msg := services.SignalingMessage{
		Type:    "candidate",
		From:    req.FromClientID,
		To:      req.ToClientID,
		Payload: payload,
	}

	err := services.GlobalSignalingServer.SendMessage(msg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "发送候选失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "候选已发送",
	})
}

// GetSignalingMessages 获取信令消息
func GetSignalingMessages(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	services.InitSignalingServer()

	msg, err := services.GlobalSignalingServer.GetMessage()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "获取消息失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": msg,
	})
}

// UnregisterSignalingClient 注销信令客户端
func UnregisterSignalingClient(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	services.InitSignalingServer()

	clientID := c.PostForm("client_id")
	if clientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "client_id参数缺失"})
		return
	}

	err := services.GlobalSignalingServer.UnregisterClient(clientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "注销失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "客户端已注销",
	})
}
