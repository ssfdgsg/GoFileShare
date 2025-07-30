package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// AuthRequired Session中间件验证
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		user := session.Get("user")
		
		fmt.Printf("AuthRequired检查，用户: %v\n", user)

		if user == nil {
			fmt.Printf("用户未登录，重定向到登录页面\n")
			c.Redirect(http.StatusFound, "/login.html")
			c.Abort()
			return
		}

		// 将用户信息传递给下一个处理函数
		c.Set("username", user)
		c.Next()
	}
}
