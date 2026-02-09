package handlers

import (
	"net/http"

	"GoFileShare/internal/service"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	userService *service.UserService
}

func NewAuthHandler(userService *service.UserService) *AuthHandler {
	return &AuthHandler{userService: userService}
}

// ShowLoginPage 显示登录页面
func (h *AuthHandler) ShowLoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "用户登录",
	})
}

// ShowRegisterPage 显示注册页面
func (h *AuthHandler) ShowRegisterPage(c *gin.Context) {
	c.HTML(http.StatusOK, "register.html", gin.H{
		"title": "用户注册",
	})
}

// Register 用户注册
func (h *AuthHandler) Register(c *gin.Context) {
	username := c.PostForm("user")
	password := c.PostForm("password")
	email := c.PostForm("email")
	if username == "" || password == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "用户名和密码不能为空",
		})
		return
	}

	exists, err := h.userService.UserExists(c.Request.Context(), username)
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

	if err := h.userService.CreateUser(c.Request.Context(), username, password, email); err != nil {
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

func (h *AuthHandler) Login(c *gin.Context) {
	username := c.PostForm("user")
	password := c.PostForm("password")
	valid, err := h.userService.ValidateUser(c.Request.Context(), username, password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "数据库错误，请稍后再试",
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

	user, err := h.userService.GetUserByName(c.Request.Context(), username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "获取用户信息失败，请联系管理员",
		})
		return
	}

	session := sessions.Default(c)
	session.Set("user", username)
	session.Set("authLevel", user.Status)
	if err := session.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Session保存失败",
		})
		return
	}

	if err := h.userService.UpdateLastLogin(c.Request.Context(), username); err != nil {
		// log-only warning
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "登录成功",
	})
}

// Logout 用户注销
func (h *AuthHandler) Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	c.Redirect(http.StatusFound, "/login.html")
}
