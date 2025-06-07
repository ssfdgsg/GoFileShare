package controllers

import (
	"fmt"
	"net/http"
	"time"

	"GoFileShare/models"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// ShowHomePage 显示主页
func ShowHomePage(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")
	loginTime := session.Get("login_time")

	fmt.Printf("访问主页，用户: %v, 登录时间: %v\n", username, loginTime)

	c.HTML(http.StatusOK, "home.html", gin.H{
		"title":       "主页",
		"username":    username,
		"currentTime": time.Now().Format("2006-01-02 15:04:05"),
		"loginTime":   loginTime,
	})
}

// GetUserInfo 获取用户信息API
func GetUserInfo(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("user")

	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未登录",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"username": username,
		"status":   "success",
	})
}

// GetUserByName 根据用户名获取用户详细信息
func GetUserByName(c *gin.Context) {
	username := c.Param("name")

	user, err := models.GetUserByName(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	if user == nil {
		c.JSON(http.StatusOK, gin.H{
			"user":   username,
			"status": "no value",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": username,
		"data": user,
	})
}
