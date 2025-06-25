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
	r.Use(sessions.Sessions("mysession", store))

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

		// API路由
		private.GET("/api/username", controllers.GetUserInfo)
		private.GET("/api/user/:name", controllers.GetUserByName)
		// 注销
		private.GET("/logout", controllers.Logout)

	}

	return r
}
