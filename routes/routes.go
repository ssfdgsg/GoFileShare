package routes

import (
	"net/http"

	"GoFileShare/controllers"
	"GoFileShare/middleware"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// SetupRouter 设置路由
func SetupRouter() *gin.Engine {
	r := gin.Default()

	// 设置Session中间件
	store := cookie.NewStore([]byte("secret-key-change-in-production"))
	store.Options(sessions.Options{
		MaxAge:   60 * 60 * 24, // 24小时过期
		HttpOnly: true,         // 防止XSS攻击
		Secure:   false,        // 生产环境设置为true（需要HTTPS）
		Path:     "/",          // 确保cookie在整个站点有效
	})
	r.Use(sessions.Sessions("session", store))

	// Ping测试
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// 公共路由（不需要登录）
	public := r.Group("/")
	{
		// 页面路由
		public.GET("/", controllers.ShowLoginPage)
		public.GET("/login.html", controllers.ShowLoginPage)
		public.GET("/register.html", controllers.ShowRegisterPage)

		// API路由
		public.POST("/api/register", controllers.Register)
		public.POST("/api/login", controllers.Login)
	}

	// 需要登录的路由
	private := r.Group("/")
	private.Use(middleware.AuthRequired())
	{
		// 页面路由
		private.GET("/home", controllers.ShowHomePage)
		private.GET("/p2p-debug", controllers.ShowP2PDebugPage)

		// API路由
		private.GET("/api/username", controllers.GetUserInfo)
		private.GET("/api/user/:name", controllers.GetUserByName)
		// 注销
		private.GET("/logout", controllers.Logout)
		// 文件操作
		private.POST("/api/InitDownloadTask/:id", controllers.InitDownloadTask)
		private.GET("/api/listFileDirByName/:name", controllers.ListFileDirByName)
		private.GET("/api/downloadFile/:id", controllers.StartDownload) // 改为GET方法
		private.POST("/api/updateFile/:id", controllers.StartUpload)
		private.GET("/api/listFileDirByID/:id", controllers.ListFileDirByID)
		private.POST("/api/updateDir/:id", controllers.UpdateDir)
		// 搜索功能
		private.GET("/api/searchFiles", controllers.SearchFiles)
		// 删除功能
		private.DELETE("/api/deleteFile/:id", controllers.DeleteFile)
		// P2P功能
		private.POST("/api/p2p/register", controllers.RegisterP2PKey)
		private.GET("/api/p2p/query", controllers.QueryP2PIP)
		private.POST("/api/p2p/connect", controllers.ConnectP2P)
		private.POST("/api/p2p/test", controllers.TestP2PConnection)
		private.GET("/api/p2p/status", controllers.GetP2PStatus)

		// 混合P2P功能（WebRTC + QUIC）
		private.POST("/api/p2p/hybrid-connect", controllers.ConnectP2PHybrid)
		private.GET("/api/p2p/methods", controllers.GetP2PConnectionMethods)
		private.POST("/api/p2p/test-methods", controllers.TestP2PMethods)
		private.GET("/api/p2p/info", controllers.QueryP2PConnectionInfo)

		// WebRTC功能
		private.POST("/api/webrtc/init", controllers.InitWebRTC)
		private.POST("/api/webrtc/offer", controllers.CreateWebRTCOffer)
		private.POST("/api/webrtc/answer", controllers.HandleWebRTCAnswer)
		private.POST("/api/webrtc/remote-offer", controllers.HandleWebRTCRemoteOffer)
		private.POST("/api/webrtc/candidate", controllers.AddWebRTCCandidate)
		private.GET("/api/webrtc/candidates", controllers.GetWebRTCCandidates)
		private.GET("/api/webrtc/status", controllers.GetWebRTCStatus)
		private.POST("/api/webrtc/message", controllers.SendWebRTCMessage)
		private.POST("/api/webrtc/close", controllers.CloseWebRTC)

		// WebRTC信令功能
		private.POST("/api/signaling/register", controllers.RegisterSignalingClient)
		private.GET("/api/signaling/client-info", controllers.GetSignalingClientInfo)
		private.POST("/api/signaling/offer", controllers.ExchangeWebRTCOffer)
		private.POST("/api/signaling/answer", controllers.ExchangeWebRTCAnswer)
		private.POST("/api/signaling/candidate", controllers.ExchangeICECandidate)
		private.GET("/api/signaling/messages", controllers.GetSignalingMessages)
		private.POST("/api/signaling/unregister", controllers.UnregisterSignalingClient)
	}

	return r
}
