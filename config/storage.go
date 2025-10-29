package config

import (
	"database/sql"
	"fmt"
	"github.com/donnie4w/go-logger/logger"
	"github.com/fatih/color"
	_ "modernc.org/sqlite"
	"os"
)

type StorageLocation struct {
	SystemFilePath string `json:"system_file_path"` // 系统文件路径，指向具体的存储位置
	NetFilePath    string `json:"net_file_path"`    // 网络文件路径，当系统路径存在的时候，此字段可以为空
}

// FileNode 代表一个逻辑上的文件或文件夹节点
type FileNode struct {
	// --- 核心标识与层级 ---
	ID       int64  `json:"id"`
	ParentID *int64 `json:"parent_id"` // 使用指针以支持NULL
	Type     bool   `json:"type"`      // 节点类型: "file":false 或 "directory":true
	Name     string `json:"name"`      // 用户看到的、在当前层级下的名称
	Path     string `json:"path"`
	//AuthLevel          *int               `json:"auth_level,omitempty"` // 权限级别，表示当前节点的权限要求，用指针表示父节点,nil表示继承父节点权限，0表示无权限
	EffectiveAuthLevel int              `json:"auth_level"`        //查询时访问的值，前端显示为auth_level
	Storage            *StorageLocation `json:"storage,omitempty"` // 存储位置，指向具体的存储节点
}

var FileDB *sql.DB
var RootPath = "." // 根目录路径

func InitFileDB() error {
	// 确保存储目录存在
	if err := os.MkdirAll("./data", 0755); err != nil {
		logger.Fatal("创建数据目录失败: ", err)
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	var err error
	// 使用SQLite数据库文件
	FileDB, err = sql.Open("sqlite", "./data/files.db")
	if err != nil {
		logger.Fatal("连接SQLite失败: ", err)
		color.Red("连接SQLite失败: %v", err)
		return err
	}

	// 测试连接
	if err = FileDB.Ping(); err != nil {
		logger.Fatal("SQLite连接测试失败: ", err)
		color.Red("SQLite连接测试失败: %v", err)
		return err
	}

	// 启用外键支持
	if _, err = FileDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		logger.Fatal("启用外键失败: ", err)
		color.Red("启用外键失败: %v", err)
		return err
	}

	// 创建文件节点表
	createTableSQL := `
    CREATE TABLE IF NOT EXISTS file_nodes (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        parent_id INTEGER,
        type INTEGER NOT NULL DEFAULT 0,
        name TEXT NOT NULL,
        path TEXT,
        effective_auth_level INTEGER NOT NULL DEFAULT 0,
        system_file_path TEXT,
        net_file_path TEXT,
        FOREIGN KEY (parent_id) REFERENCES file_nodes(id) ON DELETE CASCADE
    );
    
    CREATE INDEX IF NOT EXISTS idx_parent_id ON file_nodes(parent_id);
    CREATE INDEX IF NOT EXISTS idx_name ON file_nodes(name);
    `

	if _, err = FileDB.Exec(createTableSQL); err != nil {
		logger.Fatal("创建文件节点表失败: ", err)
		color.Red("创建文件节点表失败: %v", err)
		return err
	}

	color.Green("Connected to SQLite successfully.")
	return nil
}

func CloseFileDB() error {
	if FileDB != nil {
		err := FileDB.Close()
		if err != nil {
			logger.Error("Error disconnecting from SQLite: %v", err)
			color.Red("Error disconnecting from SQLite: %v", err)
			return err
		}
		logger.Info("Disconnected from SQLite successfully.")
		color.Green("Disconnected from SQLite successfully.")
	}
	return nil
}

func GetSystemFilePath(path string, rootPath string) string {
	SystemPath := rootPath + "/" + path
	_, err := os.Stat(SystemPath)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(SystemPath, 0755)
			if err != nil {
				logger.Error("Failed to create system file path: %v", err)
				color.Red("Failed to create system file path: %v", err)
				return ""
			}
			color.Green("Created system file path: %s", SystemPath)
		} else {
			logger.Error("Error checking system file path: %v", err)
			color.Red("Error checking system file path: %v", err)
			return ""
		}
	} else {
		color.Green("System file path already exists: %s", SystemPath)
	}
	return SystemPath
}
