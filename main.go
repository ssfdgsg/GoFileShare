package main

import (
	"fmt"
	"log"

	"GoFileShare/config"
	"GoFileShare/routes"
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

	// 设置路由
	r := routes.SetupRouter()

	// 加载HTML模板
	r.LoadHTMLGlob("views/*.html")

	fmt.Println("服务器启动在 http://127.0.0.1:8080")
	if err := r.Run("127.0.0.1:8080"); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
