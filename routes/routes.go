package routes

import (
	"net/http"

	"GoFileShare/controllers"

	"github.com/gin-gonic/gin"
)

// SetupRouter 设置路由
func SetupRouter() *gin.Engine {
	r := gin.Default()

	// Ping测试
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// 公共路由（无需登录）
	r.GET("/", controllers.ShowHomePage)
	r.GET("/home", controllers.ShowHomePage)
	r.GET("/p2p-debug", controllers.ShowP2PDebugPage)

	// 文件操作 API
	r.POST("/api/InitDownloadTask/:id", controllers.InitDownloadTask)
	r.GET("/api/listFileDirByName/:name", controllers.ListFileDirByName)
	r.GET("/api/downloadFile/:id", controllers.StartDownload)
	r.POST("/api/updateFile/:id", controllers.StartUpload)
	r.GET("/api/listFileDirByID/:id", controllers.ListFileDirByID)
	r.POST("/api/updateDir/:id", controllers.UpdateDir)

	// 搜索功能
	r.GET("/api/searchFiles", controllers.SearchFiles)

	// 删除功能
	r.DELETE("/api/deleteFile/:id", controllers.DeleteFile)

	// P2P功能
	r.POST("/api/p2p/register", controllers.RegisterP2PKey)
	r.GET("/api/p2p/query", controllers.QueryP2PIP)
	r.POST("/api/p2p/connect", controllers.ConnectP2P)
	r.POST("/api/p2p/test", controllers.TestP2PConnection)
	r.GET("/api/p2p/status", controllers.GetP2PStatus)

	return r
}
