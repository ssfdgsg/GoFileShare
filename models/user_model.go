package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User 用户模型
type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"-"`
	Email     string    `json:"email"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

var db *sql.DB

// SetDB 设置数据库连接
func SetDB(database *sql.DB) {
	db = database
}

// UserExists 检查用户是否存在
func UserExists(ctx context.Context, username string) (bool, error) {
	var count int
	query := "SELECT COUNT(*) FROM users WHERE username = ?"
	err := db.QueryRowContext(ctx, query, username).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateUser 创建新用户
func CreateUser(ctx context.Context, username, password, email string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	query := "INSERT INTO users (username, password, email, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)"
	_, err = db.ExecContext(ctx, query, username, string(hashedPassword), email, 0, time.Now(), time.Now())
	return err
}

// ValidateUser 验证用户登录
func ValidateUser(ctx context.Context, username, password string) (bool, error) {
	var hashedPassword string
	query := "SELECT password FROM users WHERE username = ?"
	err := db.QueryRowContext(ctx, query, username).Scan(&hashedPassword)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil, nil
}

// UpdateLastLogin 更新用户最后登录时间
func UpdateLastLogin(ctx context.Context, username string) error {
	query := "UPDATE users SET updated_at = ? WHERE username = ?"
	_, err := db.ExecContext(ctx, query, time.Now(), username)
	return err
}

// GetUserByName 根据用户名获取用户信息
func GetUserByName(ctx context.Context, username string) (*User, error) {
	user := &User{}
	query := "SELECT id, username, email, status, created_at, updated_at FROM users WHERE username = ?"
	err := db.QueryRowContext(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.Status, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}
