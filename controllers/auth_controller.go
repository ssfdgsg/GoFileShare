package controllers

import (
	"GoFileShare/models"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"net/http"
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
	email := c.PostForm("email")
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
	if err := models.CreateUser(username, password, email); err != nil {
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

func Login(c *gin.Context) {
	username := c.PostForm("user")
	password := c.PostForm("password")
	// 验证用户
	valid, err := models.ValidateUser(username, password)
	if err != nil {
		// Log the error for server-side debugging
		fmt.Printf("[Login Error] ValidateUser failed for %s: %v\n", username, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "数据库错误，请稍后再试", // 更通用和友好的错误信息
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

	user, err := models.GetUserByName(username)
	if err != nil {
		// !!! FIXED AREA !!!
		// 即使 GetUserByName 返回错误，也要发送 JSON 响应
		fmt.Printf("[Login Error] GetUserByName failed for %s: %v\n", username, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "获取用户信息失败，请联系管理员", // 更具体的错误信息
		})
		return // 确保在此处返回，避免后续逻辑执行
	}

	// 登录成功，创建session
	session := sessions.Default(c)
	session.Set("user", username)
	session.Set("authLevel", user.Status)

	if err := session.Save(); err != nil {
		fmt.Printf("[Login Error] Session save failed for %s: %v\n", username, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Session保存失败",
		})
		return
	}

	// 更新最后登录时间
	if err := models.UpdateLastLogin(username); err != nil {
		fmt.Printf("[Login Warning] Failed to update last login for %s: %v\n", username, err)
		// 通常这里是警告，不中断登录流程，但可以在日志中记录
	}

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
