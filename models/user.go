package models

import (
	"GoFileShare/config"
	"database/sql"
	"time"
)

type User struct {
	ID         int        `json:"id"`
	Name       string     `json:"name"`
	Password   string     `json:"password"`
	Email      string     `json:"email"`
	CreateTime time.Time  `json:"create_time"`
	LastLogin  *time.Time `json:"last_login"`
	Status     int        `json:"status"`
}

// UserExists 检查用户是否存在
func UserExists(username string) (bool, error) {
	var count int
	err := config.DB.QueryRow("SELECT COUNT(*) FROM user WHERE name = ?", username).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateUser 创建新用户
func CreateUser(username, password, email string) error {
	_, err := config.DB.Exec("INSERT INTO user(name, password, email,status) VALUES(?, ?, ?, ?)", username, password, email, 100)
	return err
}

// ValidateUser 验证用户登录
func ValidateUser(username, password string) (bool, error) {
	var count int
	err := config.DB.QueryRow("SELECT COUNT(*) FROM user WHERE name = ? AND password = ? ", username, password).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UpdateLastLogin 更新用户最后登录时间
func UpdateLastLogin(username string) error {
	_, err := config.DB.Exec("UPDATE user SET last_login = NOW() WHERE name = ?", username)
	return err
}

// GetUserByName 根据用户名获取用户信息
func GetUserByName(username string) (*User, error) {
	user := &User{}
	err := config.DB.QueryRow("SELECT id, name, password, email, create_time, last_login, status FROM user WHERE name = ?", username).Scan(
		&user.ID, &user.Name, &user.Password, &user.Email, &user.CreateTime, &user.LastLogin, &user.Status,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}
