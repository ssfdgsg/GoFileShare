package main

import (
	"context"
	"fmt"
	"log"

	"GoFileShare/config"
	"GoFileShare/models"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func main() {
	// 加载环境变量
	if err := godotenv.Load(".env"); err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	// 加载应用配置
	appConfig := config.LoadAppConfig()

	// 初始化MySQL数据库
	if err := config.InitDB(); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer config.CloseDB()

	if err := config.InitTable(); err != nil {
		log.Fatalf("初始化数据表失败: %v", err)
	}

	// 设置用户模型数据库连接
	models.SetDB(config.DB)

	// 初始化MongoDB文件数据库
	if err := config.InitFileDB(); err != nil {
		log.Fatalf("初始化文件系统链接错误: %v", err)
	}

	// 设置文件模型数据库连接
	models.SetFileCollection(config.FileCollection, appConfig.RootPath)

	// 初始化P2P管理器
	models.InitP2PManager(appConfig.ListenPort, appConfig.QUICPort, appConfig.STUNServers)

	// 初始化根文件节点
	rootAuthLevel := 100
	result, err := models.SearchFileNodeByName(context.Background(), "root")
	if err != nil {
		log.Fatal(err)
	}
	if len(result) == 0 {
		if err := models.AddFileNode(context.Background(), "./FileStore", "root", false, primitive.NewObjectID().String(), rootAuthLevel); err != nil {
			log.Fatal(err)
		}
	}

	// 设置路由
	r := SetupRouter(appConfig.RootPath, appConfig.P2PServerIP, appConfig.P2PServerPort)
	r.LoadHTMLGlob("views/*.html")

	// 启动服务器
	fmt.Println("服务器启动在 http://localhost:8080")
	if err := r.Run("0.0.0.0:8080"); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
