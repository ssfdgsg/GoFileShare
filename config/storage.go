package config

import (
	"context"
	"fmt"
	"github.com/donnie4w/go-logger/logger"
	"github.com/fatih/color"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"os"
)

type StorageLocation struct {
	SystemFilePath string `bson:"system_file_path"` // 系统文件路径，指向具体的存储位置
	NetFilePath    string `bson:"net_file_path"`    // 网络文件路径，当系统路径存在的时候，此字段可以为空
}

// FileNode 代表一个逻辑上的文件或文件夹节点
type FileNode struct {
	// --- 核心标识与层级 ---
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	ParentID primitive.ObjectID `bson:"parent_id,omitempty" json:"parent_id"`
	Type     bool               `bson:"type" json:"type"` // 节点类型: "file":false 或 "directory":true
	Name     string             `bson:"name" json:"name"` // 用户看到的、在当前层级下的名称，�� "report.pdf" 或 "documents"
	Path     string             `bson:"path" json:"path"`
	//AuthLevel          *int               `bson:"auth_level,omitempty"` // 权限级别，表示当前节点的权限要求，用指针表示父节点,nil表示继承父节点权限，0表示无权限
	EffectiveAuthLevel int              `bson:"effective_auth_level" json:"auth_level"`     //查询时访问的值，前端显示为auth_level
	Storage            *StorageLocation `bson:"storage,omitempty" json:"storage,omitempty"` // 存储位置，指向具体的存储节点'
}

var FileClient *mongo.Client
var FileCollection *mongo.Collection
var RootPath = "." // 根目录路径

func InitFileDB() error {
	// 加载 .env 文件
	if err := godotenv.Load(); err != nil {
		logger.Fatal("加载 .env 文件失败: ", err)
		return fmt.Errorf("加载 .env 文件失败: %w", err)
	}

	var err error
	mongoURL := os.Getenv("MONGO_URL")
	mongoUser := os.Getenv("MONGO_USER")
	mongoPassword := os.Getenv("MONGO_PASSWORD")
	mongoHost := os.Getenv("MONGO_HOST")
	mongoPort := os.Getenv("MONGO_PORT")

	if mongoURL == "" && (mongoUser == "" || mongoPassword == "" || mongoHost == "" || mongoPort == "") {
		logger.Fatal("MongoDB环境变量未正确设置: MONGO_USER, MONGO_PASSWORD, MONGO_HOST, MONGO_PORT")
		return fmt.Errorf("MongoDB环境变量未正确设置")
	}

	connectURL := "mongodb://" + mongoUser + ":" + mongoPassword + "@" + mongoHost + ":" + mongoPort
	if mongoURL != "" {
		connectURL = mongoURL
	}
	FileClient, err = mongo.Connect(context.TODO(), options.Client().ApplyURI(connectURL))
	if err != nil {
		logger.Fatal(err)
		color.Red("Fail to connect to MongoDB: %v", err)
		return err
	}
	err = FileClient.Ping(context.TODO(), nil)
	if err != nil {
		logger.Fatal(err)
		color.Red("Fail to ping MongoDB: %v", err)
		return err
	}
	FileCollection = FileClient.Database("GoFileShare").Collection("FileDir")

	color.Green("Connected to MongoDB successfully.")

	return nil
}

func CloseFileDB() error {
	if FileClient != nil {
		err := FileClient.Disconnect(context.TODO())
		if err != nil {
			logger.Error("Error disconnecting from MongoDB: %v", err)
			color.Red("Error disconnecting from MongoDB: %v", err)
			return err
		}
		logger.Info("Disconnected from MongoDB successfully.")
		color.Green("Disconnected from MongoDB successfully.")
	}
	return nil
}

func ParseObjectID(id string) (primitive.ObjectID, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		logger.Error("Invalid ObjectID: %v", err)
		color.Red("Invalid ObjectID: %v", err)
		return primitive.NilObjectID, err
	}
	return objectID, nil
}

func GetSystemFilePath(path string, rootPath string) string {
	SystemPath := path + rootPath
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
