package controllers

import (
    "GoFileShare/services"
    "net/http"

    "github.com/gin-contrib/sessions"
    "github.com/gin-gonic/gin"
)

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
func ConnectP2P(c *gin.Context) {
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

    // 确保全局P2P客户端已初始化
    if services.GlobalP2PClient == nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "P2P服务未初始化"})
        return
    }

    // 获取打洞信息
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

    // 创建目标客户端并建立连接
    targetClient := &services.TargetClient{
        ExternalIP:   holePunchInfo.ExternalIP,
        ExternalPort: holePunchInfo.ExternalPort,
        LocalIP:      holePunchInfo.LocalIP,
        LocalPort:    holePunchInfo.LocalPort,
    }

    // 异步建立P2P连接
    go func() {
        targetClient.EchoPeer()
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
func TestP2PConnection(c *gin.Context) {
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

    // 确保全局P2P客户端已初始化
    if services.GlobalP2PClient == nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "P2P服务未初始化"})
        return
    }

    // 获取打洞信息
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

    // 测试连接
    err = services.GlobalP2PClient.ConnectPeerTest(holePunchInfo)
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
func GetP2PStatus(c *gin.Context) {
    session := sessions.Default(c)
    username := session.Get("user")
    if username == nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
        return
    }

    if services.GlobalP2PClient == nil {
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
            "initialized":   true,
            "key":           services.GlobalP2PClient.Key,
            "external_ip":   services.GlobalP2PClient.OutIP,
            "external_port": services.GlobalP2PClient.OutPort,
        },
    })
}
