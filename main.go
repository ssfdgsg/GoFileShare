package main

import (
	"GoFileShare/config"
	"GoFileShare/models"
	"GoFileShare/routes"
	"fmt"
	"github.com/donnie4w/go-logger/logger"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	_ "net/http/pprof"
)

var (
	FreePort = 8080 // 默认端口
)

func main() {
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

	// 设置路由
	r := routes.SetupRouter()

	// 加载HTML模板
	r.LoadHTMLGlob("views/*.html")

	fmt.Println("服务器启动在 http://0.0.0.0:8080")
	if err := r.Run("0.0.0.0:8080"); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
