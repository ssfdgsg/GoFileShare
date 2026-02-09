package main

import (
	"net/http"

	"GoFileShare/controllers"
	"GoFileShare/middleware"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// SetupRouter 设置路由
func SetupRouter(rootPath, p2pServerIP, p2pServerPort string) *gin.Engine {
	r := gin.Default()

	// Session配置
	store := cookie.NewStore([]byte("secret-key-change-in-production"))
	store.Options(sessions.Options{
		MaxAge:   60 * 60 * 24,
		HttpOnly: true,
		Secure:   false,
		Path:     "/",
	})
	r.Use(sessions.Sessions("session", store))

	// 健康检查
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// 初始化控制器
	authCtrl := controllers.NewAuthController()
	fileCtrl := controllers.NewFileController(rootPath)
	p2pCtrl := controllers.NewP2PController(p2pServerIP, p2pServerPort)

	// 公开路由
	public := r.Group("/")
	{
		public.GET("/", authCtrl.ShowLoginPage)
		public.GET("/login.html", authCtrl.ShowLoginPage)
		public.GET("/register.html", authCtrl.ShowRegisterPage)
		public.POST("/api/register", authCtrl.Register)
		public.POST("/api/login", authCtrl.Login)
	}

	// 需要认证的路由
	private := r.Group("/")
	private.Use(middleware.AuthRequired())
	{
		// 页面路由
		private.GET("/home", fileCtrl.ShowHomePage)
		private.GET("/p2p-debug", p2pCtrl.ShowP2PDebugPage)

		// 用户API
		private.GET("/api/username", fileCtrl.GetUserInfo)
		private.GET("/api/user/:name", fileCtrl.GetUserByName)
		private.GET("/logout", authCtrl.Logout)

		// 文件API
		private.POST("/api/InitDownloadTask/:id", fileCtrl.InitDownloadTask)
		private.GET("/api/listFileDirByName/:name", fileCtrl.ListFileDirByName)
		private.GET("/api/downloadFile/:id", fileCtrl.StartDownload)
		private.POST("/api/updateFile/:id", fileCtrl.StartUpload)
		private.GET("/api/listFileDirByID/:id", fileCtrl.ListFileDirByID)
		private.POST("/api/updateDir/:id", fileCtrl.UpdateDir)
		private.GET("/api/searchFiles", fileCtrl.SearchFiles)
		private.DELETE("/api/deleteFile/:id", fileCtrl.DeleteFile)

		// P2P API
		private.POST("/api/p2p/register", p2pCtrl.RegisterP2PKey)
		private.GET("/api/p2p/query", p2pCtrl.QueryP2PIP)
		private.POST("/api/p2p/connect", p2pCtrl.ConnectP2P)
		private.POST("/api/p2p/test", p2pCtrl.TestP2PConnection)
		private.GET("/api/p2p/status", p2pCtrl.GetP2PStatus)
	}

	return r
}
