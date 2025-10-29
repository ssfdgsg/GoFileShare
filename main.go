package main

import (
	"GoFileShare/config"
	"GoFileShare/routes"
	"fmt"
	"log"
	"net"
	_ "net/http/pprof"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("警告：加载配置文件失败（非必需）: %v", err)
	}

	// 初始化文件数据库（SQLite）
	if err := config.InitFileDB(); err != nil {
		log.Fatalf("初始化文件系统链接错误: %v", err)
	} else {
		log.Println("初始化文件系统成功")
	}
	defer config.CloseFileDB()

	if err := os.MkdirAll("./FileStore", 0755); err != nil {
		log.Fatalf("创建文件存储目录失败: %v", err)
	}

	// 设置路由
	r := routes.SetupRouter()

	// 加载HTML模板
	r.LoadHTMLGlob("views/*.html")

	// 显示局域网访问地址
	fmt.Println("==========================================")
	fmt.Println("🚀 GoFileShare 文件共享服务已启动")
	fmt.Println("==========================================")
	fmt.Println("本地访问: http://localhost:8080")

	// 获取局域网IP地址
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		fmt.Println("\n局域网访问地址:")
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					fmt.Printf("  http://%s:8080\n", ipnet.IP.String())
				}
			}
		}
	}
	fmt.Println("==========================================\n")

	if err := r.Run("0.0.0.0:8080"); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
