package main

import (
	"GoFileShare/config"
	"GoFileShare/models"
	"GoFileShare/routes"
	"GoFileShare/services"
	"fmt"
	"github.com/donnie4w/go-logger/logger"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	_ "net/http/pprof"
	"os"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}
	// 初始化数据库连接
	if err := config.InitDB(); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer config.CloseDB()

	// 初始化数据库表
	if err := config.InitTable(); err != nil {
		log.Fatalf("初始化数据表失败: %v", err)
	}

	if err := config.InitFileDB(); err != nil {
		log.Fatalf("初始化文件系统链接错误: %v", err)
	} else {
		log.Println("初始化文件系统成功")
	}
	var RootAuthLevel int
	RootAuthLevel = 100

	result, err := models.SearchFileNodeByName("root")
	if err != nil {
		log.Fatal(err)
	}
	if len(result) == 0 {
		err := models.AddFileNode("./FileStore", "root", false, primitive.NewObjectID().String(), RootAuthLevel)
		if err != nil {
			logger.Fatal(err)
		}
	}

	// 初始化P2P客户端
	serverAddr := os.Getenv("P2P_SERVER_IP") + ":" + os.Getenv("P2P_SERVER_PORT")
	err = services.InitP2PClient(serverAddr)
	if err != nil {
		log.Printf("P2P客户端初始化失败: %v", err)
	} else {
		// 注册到P2P服务器
		p2pClient := services.GetGlobalP2PClient()
		if p2pClient != nil {
			// 修复调用，移除多余的参数
			err = p2pClient.Register()
			if err != nil {
				log.Printf("P2P注册失败: %v", err)
			} else {
				log.Println("P2P客户端注册成功")
			}
		}
	}

	// 设置路由
	r := routes.SetupRouter()

	// 加载HTML模板
	r.LoadHTMLGlob("views/*.html")

	fmt.Println("服务器启动在 http://0.0.0.0:8080")
	if err := r.Run("0.0.0.0:8080"); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
