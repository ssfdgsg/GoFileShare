package config

import (
	"GoFileShare/utils"
	"database/sql"
	"fmt"
	"github.com/fatih/color"
	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	"time"
)

// FileNode 接口定义（用于权限检查）
type FileNode interface {
	GetAuthLevel() int
}

var DB *sql.DB

// InitDB 初始化数据库连接
func InitDB() error {
	dbHost := utils.GetEnv("DB_HOST", "localhost") // 默认是本地，方便开发
	dbPort := utils.GetEnv("DB_PORT", "3306")
	dbUser := utils.GetEnv("DB_USER", "root")
	dbPassword := utils.GetEnv("DB_PASSWORD", "123456") // 注意：这是你原来的密码
	dbName := utils.GetEnv("DB_NAME", "gotest")
	cfg := mysql.Config{
		User:                 dbUser,
		Passwd:               dbPassword,
		Net:                  "tcp",
		Addr:                 dbHost + ":" + dbPort,
		DBName:               dbName,
		AllowNativePasswords: true,
		ParseTime:            true,
	}
	var err error
	DB, err = sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return fmt.Errorf("sql.Open失败: %w", err)
	}

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("数据库连接测试失败: %w", err)
	}

	color.Green("连接MySQL数据库成功~")

	// 设置数据库连接池参数
	DB.SetMaxOpenConns(10)
	DB.SetMaxIdleConns(5)
	DB.SetConnMaxLifetime(time.Minute * 3)

	return nil
}

// InitTable 初始化数据库表
func InitTable() error {
	createTableSQL := `
    CREATE TABLE IF NOT EXISTS users (
        id INT AUTO_INCREMENT PRIMARY KEY,
        username VARCHAR(100) NOT NULL UNIQUE,
        password VARCHAR(255) NOT NULL,
        email VARCHAR(255),
        status INT DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`

	_, err := DB.Exec(createTableSQL)
	return err
}

// CloseDB 关闭数据库连接
func CloseDB() {
	if DB != nil {
		DB.Close()
	}
}

func AuthCheck[T any](AuthLevel int, FileNodes []T) ([]T, error) {
	var filteredNodes []T
	for _, node := range FileNodes {
		// 使用类型断言获取AuthLevel字段
		if v, ok := any(node).(interface{ GetAuthLevel() int }); ok {
			if v.GetAuthLevel() <= AuthLevel {
				filteredNodes = append(filteredNodes, node)
			}
		}
	}
	if len(filteredNodes) == 0 {
		return nil, nil
	}
	return filteredNodes, nil
}
