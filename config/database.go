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

var DB *sql.DB

// InitDB 初始化数据库连接
func InitDB() error {
	dbHost := utils.GetEnv("DB_HOST", "127.0.0.1") // 默认是本地，方便开发
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
    CREATE TABLE IF NOT EXISTS user (
        id INT AUTO_INCREMENT PRIMARY KEY,
        name VARCHAR(100) NOT NULL UNIQUE,
        password VARCHAR(255) NOT NULL,
        email VARCHAR(255),
        create_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        last_login TIMESTAMP NULL,
        status TINYINT(1) DEFAULT 1
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

func AuthCheck(AuthLevel int, FileNodes []FileNode) ([]FileNode, error) {
	var filteredNodes []FileNode
	for _, node := range FileNodes {
		if node.EffectiveAuthLevel <= AuthLevel {
			filteredNodes = append(filteredNodes, node)
		}
	}
	if len(filteredNodes) == 0 {
		return nil, nil // 没有符合条件的节点
	}
	return filteredNodes, nil
}
