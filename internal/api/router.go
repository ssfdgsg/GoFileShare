package api

import (
	"net/http"

	"GoFileShare/internal/api/handlers"
	"GoFileShare/middleware"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

type RouterDeps struct {
	AuthHandler *handlers.AuthHandler
	UserHandler *handlers.UserHandler
	P2PHandler  *handlers.P2PHandler
}

func SetupRouter(deps RouterDeps) *gin.Engine {
	r := gin.Default()

	store := cookie.NewStore([]byte("secret-key-change-in-production"))
	store.Options(sessions.Options{
		MaxAge:   60 * 60 * 24,
		HttpOnly: true,
		Secure:   false,
		Path:     "/",
	})
	r.Use(sessions.Sessions("session", store))

	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	public := r.Group("/")
	{
		public.GET("/", deps.AuthHandler.ShowLoginPage)
		public.GET("/login.html", deps.AuthHandler.ShowLoginPage)
		public.GET("/register.html", deps.AuthHandler.ShowRegisterPage)
		public.POST("/api/register", deps.AuthHandler.Register)
		public.POST("/api/login", deps.AuthHandler.Login)
	}

	private := r.Group("/")
	private.Use(middleware.AuthRequired())
	{
		private.GET("/home", deps.UserHandler.ShowHomePage)
		private.GET("/p2p-debug", deps.P2PHandler.ShowP2PDebugPage)

		private.GET("/api/username", deps.UserHandler.GetUserInfo)
		private.GET("/api/user/:name", deps.UserHandler.GetUserByName)
		private.GET("/logout", deps.AuthHandler.Logout)

		private.POST("/api/InitDownloadTask/:id", deps.UserHandler.InitDownloadTask)
		private.GET("/api/listFileDirByName/:name", deps.UserHandler.ListFileDirByName)
		private.GET("/api/downloadFile/:id", deps.UserHandler.StartDownload)
		private.POST("/api/updateFile/:id", deps.UserHandler.StartUpload)
		private.GET("/api/listFileDirByID/:id", deps.UserHandler.ListFileDirByID)
		private.POST("/api/updateDir/:id", deps.UserHandler.UpdateDir)
		private.GET("/api/searchFiles", deps.UserHandler.SearchFiles)
		private.DELETE("/api/deleteFile/:id", deps.UserHandler.DeleteFile)

		private.POST("/api/p2p/register", deps.P2PHandler.RegisterP2PKey)
		private.GET("/api/p2p/query", deps.P2PHandler.QueryP2PIP)
		private.POST("/api/p2p/connect", deps.P2PHandler.ConnectP2P)
		private.POST("/api/p2p/test", deps.P2PHandler.TestP2PConnection)
		private.GET("/api/p2p/status", deps.P2PHandler.GetP2PStatus)
	}

	return r
}
