package controllers

import (
    "GoFileShare/services"
    "net/http"

    "github.com/gin-gonic/gin"
)

// RegisterP2PKey 注册P2P密钥
func RegisterP2PKey(c *gin.Context) {
    key := c.PostForm("key")
    if key == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "密钥不能为空"})
        return
    }

    // 初始化P2P服务
    p2pService, err := services.InitInfo()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":   "初始化P2P服务失败",
            "message": err.Error(),
        })
        return
    }

    // 确保服务已正确初始化
    if p2pService == nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":   "P2P服务初始化失败",
            "message": "服务对象为空",
        })
        return
    }

    err = p2pService.Register()
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
        "key":     p2pService.Key,
        "ip":      p2pService.OutIP,
        "port":    p2pService.OutPort,
    })
}

// QueryP2PIP 查询P2P密钥对应的客户端信息
func QueryP2PIP(c *gin.Context) {
    key := c.Query("key")
    if key == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "密钥不能为空"})
        return
    }
    holePunchInfo, err := services.GlobalP2PClient.GetHolePunch(key)
    if err != nil {
        c.JSON(http.StatusBadGateway, gin.H{"error": "查询失败", "message": err.Error()})
        return
    } else {
        c.JSON(http.StatusOK, gin.H{"status": "success", "data": holePunchInfo})
    }
    return
}

// ShowP2PDebugPage 显示P2P调试页面
func ShowP2PDebugPage(c *gin.Context) {
    c.HTML(http.StatusOK, "p2p_debug.html", gin.H{
        "title": "P2P调试界面",
    })
}

// ConnectP2P 连接到P2P节点
func ConnectP2P(c *gin.Context) {
    targetKey := c.PostForm("targetKey")
    if targetKey == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "目标密钥不能为空"})
        return
    }

    // 获取目标客户端信息
    holePunchInfo, err := services.GlobalP2PClient.GetHolePunch(targetKey)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error":   "查询目标客户端失败",
            "message": err.Error(),
        })
        return
    }

    if !holePunchInfo.HasRequest {
        c.JSON(http.StatusNotFound, gin.H{
            "error":   "目标客户端不存在",
            "message": "未找到对应密钥的客户端",
        })
        return
    }

    // 保存目标客户端信息
    targetIP := holePunchInfo.ExternalIP
    targetPort := holePunchInfo.ExternalPort

    // 初始化自己的P2P服务
    p2pService, err := services.InitInfo()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":   "初始化P2P服务失败",
            "message": err.Error(),
        })
        return
    }

    // 注册到服务器
    err = p2pService.Register()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":   "注册失败",
            "message": err.Error(),
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "status":    "success",
        "message":   "已获取目标信息，准备连接",
        "targetIP":  targetIP,
        "targetPort": targetPort,
        "myKey":     p2pService.Key,
        "myIP":      p2pService.OutIP,
        "myPort":    p2pService.OutPort,
    })
}

// TestP2PConnection 测试P2P连接
func TestP2PConnection(c *gin.Context) {
    targetIP := c.PostForm("targetIP")
    targetPort := c.PostForm("targetPort")

    if targetIP == "" || targetPort == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "目标IP和端口不能为空"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "status":    "success",
        "message":   "准备测试连接",
        "targetIP":  targetIP,
        "targetPort": targetPort,
    })
}

// GetP2PStatus 获取P2P状态
func GetP2PStatus(c *gin.Context) {
    p2pService, err := services.InitInfo()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":   "获取P2P状态失败",
            "message": err.Error(),
        })
        return
    }

    if p2pService == nil {
        c.JSON(http.StatusOK, gin.H{
            "status":      "not_initialized",
            "initialized": false,
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "status":       "initialized",
        "initialized":  true,
        "key":          p2pService.Key,
        "external_ip":  p2pService.OutIP,
        "external_port": p2pService.OutPort,
    })
}
