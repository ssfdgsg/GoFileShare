package server

import (
	"fmt"
	"log"
	_ "net/http/pprof"

	"GoFileShare/config"
	configinternal "GoFileShare/internal/config"
	"GoFileShare/internal/p2p/discovery"
	"GoFileShare/internal/p2p/manager"
	"GoFileShare/internal/p2p/signaling"
	"GoFileShare/internal/p2p/transport"
	mongorepo "GoFileShare/internal/repository/mongo"
	mysqlrepo "GoFileShare/internal/repository/mysql"
	"GoFileShare/internal/service"
	"GoFileShare/models"
	"GoFileShare/routes"

	"github.com/donnie4w/go-logger/logger"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Main is the entry point for the server.
func Main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	appConfig, err := configinternal.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	if err := config.InitDB(); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer config.CloseDB()

	if err := config.InitTable(); err != nil {
		log.Fatalf("初始化数据表失败: %v", err)
	}

	if err := config.InitFileDB(); err != nil {
		log.Fatalf("初始化文件系统链接错误: %v", err)
	}

	userRepo := mysqlrepo.NewUserRepository(config.DB)
	fileRepo := mongorepo.NewFileRepository(config.FileCollection, appConfig.RootPath)
	models.SetUserRepository(userRepo)
	models.SetFileRepository(fileRepo)

	userService := service.NewUserService(userRepo)
	fileService := service.NewFileService(fileRepo)

	p2pDiscovery := discovery.NewSTUNDiscovery(appConfig.P2P.STUNServers, appConfig.P2P.QUICPort)
	p2pSignaling := signaling.NewHTTPClient(appConfig.P2P.ServerIP, appConfig.P2P.ServerPort)
	p2pTransport := transport.NewUDPTransport(appConfig.P2P.ListenPort, appConfig.P2P.QUICPort)
	p2pManager := manager.NewP2PManager(p2pDiscovery, p2pSignaling, p2pTransport)
	p2pService := service.NewP2PService(p2pManager)

	rootAuthLevel := 100
	result, err := models.SearchFileNodeByName("root")
	if err != nil {
		log.Fatal(err)
	}
	if len(result) == 0 {
		if err := models.AddFileNode("./FileStore", "root", false, primitive.NewObjectID().String(), rootAuthLevel); err != nil {
			logger.Fatal(err)
		}
	}

	r := routes.SetupRouter(userService, fileService, p2pService, appConfig.RootPath)
	r.LoadHTMLGlob("views/*.html")

	fmt.Println("服务器启动在 http://localhost:8080")
	if err := r.Run("0.0.0.0:8080"); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
