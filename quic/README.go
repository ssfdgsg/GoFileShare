package quic

// 这个文件包含了如何使用新的QUIC架构的示例

/*
新的QUIC架构使用说明：

1. 初始化QUIC管理器（包含P2P功能）：
   err := quic.InitQUICWithP2P("127.0.0.1", 9001, "192.168.1.100", 9001)

2. 获取QUIC管理器实例：
   manager := quic.GetGlobalQUICManager()

3. 连接到对等节点：
   err := manager.ConnectToPeer("peer1", "192.168.1.101", 9001)

4. 发送数据：
   err := manager.SendData("192.168.1.101:9001", []byte("Hello World"))

5. 发送文件：
   file, _ := os.Open("example.txt")
   defer file.Close()
   stat, _ := file.Stat()
   err := manager.SendFile("peer1", "example.txt", file, stat.Size())

6. 发送消息：
   err := manager.SendMessage("peer1", "Hello from QUIC!")

7. 获取HTTP代理（用于Gin中间件）：
   httpProxy := manager.GetHTTPProxy()
   router.Use(httpProxy.ProxyHandler())

8. 获取状态信息：
   status := manager.GetStatus()

9. 关闭管理器：
   err := manager.Close()

主要改进：
- 统一的QUIC管理器，整合了所有QUIC功能
- 更好的错误处理和超时控制
- 支持文件大小信息的文件传输
- 线程安全的连接管理
- 简化的API接口
- 更清晰的模块化设计

架构组件：
- QUICProxy: 核心QUIC代理功能
- P2PQUICManager: P2P连接管理
- HTTPToQUICProxy: HTTP到QUIC的代理
- QUICFileTransfer: 文件传输功能
- QUICManager: 统一管理器，整合所有功能
*/
