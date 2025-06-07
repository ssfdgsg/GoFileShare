package controllers

import (
	"fmt"
	"net/http"
	"time"

	"GoFileShare/models"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// ShowLoginPage 显示登录页面
func ShowLoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "用户登录",
	})
}

// ShowRegisterPage 显示注册页面
func ShowRegisterPage(c *gin.Context) {
	c.HTML(http.StatusOK, "register.html", gin.H{
		"title": "用户注册",
	})
}

// Register 用户注册
func Register(c *gin.Context) {
	username := c.PostForm("user")
	password := c.PostForm("password")

	// 验证输入
	if username == "" || password == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "用户名和密码不能为空",
		})
		return
	}

	// 检查用户是否存在
	exists, err := models.UserExists(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "数据库查询错误",
		})
		return
	}

	if exists {
		c.JSON(http.StatusConflict, gin.H{
			"status":  "error",
			"message": "用户已存在",
		})
		return
	}

	// 创建用户
	if err := models.CreateUser(username, password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "用户创建失败",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"message": "用户创建成功",
	})
}

// Login 用户登录
func Login(c *gin.Context) {
	username := c.PostForm("user")
	password := c.PostForm("password")

	// 验证用户
	valid, err := models.ValidateUser(username, password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "数据库错误",
		})
		return
	}

	if !valid {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "用户名或密码错误",
		})
		return
	}

	// 登录成功，创建session
	session := sessions.Default(c)
	session.Set("user", username)
	session.Set("login_time", time.Now().Format("2006-01-02 15:04:05"))

	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Session保存失败",
		})
		return
	}

	// 更新最后登录时间
	models.UpdateLastLogin(username)

	fmt.Printf("用户 %s 登录成功，Session已保存\n", username)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "登录成功",
	})
}

// Logout 用户注销
func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	c.Redirect(http.StatusFound, "/login.html")
}
