package controllers

import (
	"GoFileShare/services"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// InitWebRTC 初始化WebRTC连接
func InitWebRTC(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	var webrtcService *services.WebRTCService
	var err error

	if services.GlobalWebRTCService == nil {
		webrtcService, err = services.NewWebRTCService()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "初始化WebRTC失败",
				"message": err.Error(),
			})
			return
		}
		services.GlobalWebRTCService = webrtcService

		// 创建数据通道
		_, err = webrtcService.CreateDataChannel("file-transfer")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "创建数据通道失败",
				"message": err.Error(),
			})
			return
		}
	} else {
		webrtcService = services.GlobalWebRTCService
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"message":    "WebRTC已初始化",
		"connection": webrtcService.GetConnectionState(),
	})
}

// CreateWebRTCOffer 创建WebRTC Offer
func CreateWebRTCOffer(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	if services.GlobalWebRTCService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "WebRTC服务未初始化"})
		return
	}

	offer, err := services.GlobalWebRTCService.CreateOffer()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "创建Offer失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"offer":  offer,
	})
}

// HandleWebRTCAnswer 处理WebRTC Answer
func HandleWebRTCAnswer(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	if services.GlobalWebRTCService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "WebRTC服务未初始化"})
		return
	}

	type AnswerRequest struct {
		Answer string `json:"answer" binding:"required"`
	}

	var req AnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求"})
		return
	}

	err := services.GlobalWebRTCService.HandleAnswer(req.Answer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "处理Answer失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Answer已处理",
	})
}

// HandleWebRTCRemoteOffer 处理远程Offer
func HandleWebRTCRemoteOffer(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	if services.GlobalWebRTCService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "WebRTC服务未初始化"})
		return
	}

	type OfferRequest struct {
		Offer string `json:"offer" binding:"required"`
	}

	var req OfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求"})
		return
	}

	answer, err := services.GlobalWebRTCService.HandleRemoteOffer(req.Offer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "处理Offer失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Answer已生成",
		"answer":  answer,
	})
}

// AddWebRTCCandidate 添加ICE候选
func AddWebRTCCandidate(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	if services.GlobalWebRTCService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "WebRTC服务未初始化"})
		return
	}

	type CandidateRequest struct {
		Candidate string `json:"candidate" binding:"required"`
	}

	var req CandidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求"})
		return
	}

	err := services.GlobalWebRTCService.AddICECandidate(req.Candidate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "添加候选失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "ICE候选已添加",
	})
}

// GetWebRTCCandidates 获取ICE候选列表
func GetWebRTCCandidates(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	if services.GlobalWebRTCService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "WebRTC服务未初始化"})
		return
	}

	candidates, err := services.GlobalWebRTCService.GetICECandidates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "获取候选失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"candidates": candidates,
	})
}

// GetWebRTCStatus 获取WebRTC连接状态
func GetWebRTCStatus(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	if services.GlobalWebRTCService == nil {
		c.JSON(http.StatusOK, gin.H{
			"status": "not_initialized",
			"data": gin.H{
				"initialized": false,
				"message":     "WebRTC服务未初始化",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"initialized":      true,
			"connection_state": services.GlobalWebRTCService.GetConnectionState(),
		},
	})
}

// SendWebRTCMessage 通过WebRTC发送消息
func SendWebRTCMessage(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	if services.GlobalWebRTCService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "WebRTC服务未初始化"})
		return
	}

	type MessageRequest struct {
		Label   string `json:"label" binding:"required"`
		Message string `json:"message" binding:"required"`
	}

	var req MessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求"})
		return
	}

	err := services.GlobalWebRTCService.SendMessage(req.Label, []byte(req.Message))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "发送消息失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "消息已发送",
	})
}

// CloseWebRTC 关闭WebRTC连接
func CloseWebRTC(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	if services.GlobalWebRTCService != nil {
		err := services.GlobalWebRTCService.Close()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "关闭WebRTC失败",
				"message": err.Error(),
			})
			return
		}
		services.GlobalWebRTCService = nil
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "WebRTC已关闭",
	})
}
