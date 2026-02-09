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
			logger.Errorf("Error disconnecting from MongoDB: %v", err)
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
		logger.Errorf("Invalid ObjectID: %v", err)
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
				logger.Errorf("Failed to create system file path: %v", err)
				color.Red("Failed to create system file path: %v", err)
				return ""
			}
			color.Green("Created system file path: %s", SystemPath)
		} else {
			logger.Errorf("Error checking system file path: %v", err)
			color.Red("Error checking system file path: %v", err)
			return ""
		}
	} else {
		color.Green("System file path already exists: %s", SystemPath)
	}
	return SystemPath
}
