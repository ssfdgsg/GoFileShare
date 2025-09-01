package quic

import (
	"fmt"
	"log"
	"sync"
)

// QUICManager QUIC管理器，统一管理所有QUIC相关功能
type QUICManager struct {
	proxy         *QUICProxy
	p2pManager    *P2PQUICManager
	httpProxy     *HTTPToQUICProxy
	fileTransfer  *QUICFileTransfer
	isInitialized bool
	mutex         sync.RWMutex
}

// NewQUICManager 创建新的QUIC管理器
func NewQUICManager(config *QUICConfig) (*QUICManager, error) {
	if config == nil {
		config = DefaultQUICConfig()
	}

	// 创建QUIC代理
	proxy, err := NewQUICProxy(config)
	if err != nil {
		return nil, fmt.Errorf("创建QUIC代理失败: %w", err)
	}

	// 创建HTTP代理
	httpProxy := NewHTTPToQUICProxy("localhost:8080")
	httpProxy.SetQUICProxy(proxy)

	// 创建文件传输
	fileTransfer := NewQUICFileTransfer(proxy)

	return &QUICManager{
		proxy:        proxy,
		httpProxy:    httpProxy,
		fileTransfer: fileTransfer,
	}, nil
}

// InitP2P 初始化P2P功能
func (qm *QUICManager) InitP2P(localIP string, localPort int, externalIP string, externalPort int) error {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()

	p2pManager, err := NewP2PQUICManager(localIP, localPort, externalIP, externalPort)
	if err != nil {
		return fmt.Errorf("创建P2P管理器失败: %w", err)
	}

	qm.p2pManager = p2pManager

	// 设置全局P2P QUIC管理器
	SetGlobalP2PQUICManager(p2pManager)

	// 启动监听器
	localAddr := fmt.Sprintf("%s:%d", localIP, localPort)
	if err := p2pManager.StartListener(localAddr); err != nil {
		return fmt.Errorf("启动本地监听器失败: %w", err)
	}

	// 如果外部地址不同，也启动外部监听器
	if externalIP != localIP || externalPort != localPort {
		externalAddr := fmt.Sprintf("0.0.0.0:%d", externalPort)
		if err := p2pManager.StartListener(externalAddr); err != nil {
			log.Printf("启动外部监听器失败: %v", err)
		}
	}

	log.Printf("P2P QUIC管理器初始化完成")
	return nil
}

// StartListener 启动QUIC监听器
func (qm *QUICManager) StartListener(addr string) error {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()

	if qm.proxy == nil {
		return fmt.Errorf("QUIC代理未初始化")
	}

	return qm.proxy.StartListener(addr)
}

// ConnectToPeer 连接到对等节点
func (qm *QUICManager) ConnectToPeer(peerKey, peerIP string, peerPort int) error {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()

	if qm.p2pManager == nil {
		return fmt.Errorf("P2P管理器未初始化")
	}

	return qm.p2pManager.ConnectToPeer(peerKey, peerIP, peerPort)
}

// SendData 发送数据到指定地址
func (qm *QUICManager) SendData(addr string, data []byte) error {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()

	if qm.proxy == nil {
		return fmt.Errorf("QUIC代理未初始化")
	}

	return qm.proxy.SendData(addr, data)
}

// SendFile 发送文件
func (qm *QUICManager) SendFile(targetPeer, filePath string, fileData interface{}, fileSize int64) error {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()

	if qm.fileTransfer == nil {
		return fmt.Errorf("文件传输未初始化")
	}

	// 类型断言，确保fileData实现了io.Reader接口
	reader, ok := fileData.(interface{ Read([]byte) (int, error) })
	if !ok {
		return fmt.Errorf("fileData必须实现io.Reader接口")
	}

	return qm.fileTransfer.SendFile(targetPeer, filePath, reader, fileSize)
}

// SendMessage 发送消息
func (qm *QUICManager) SendMessage(targetPeer, message string) error {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()

	if qm.fileTransfer == nil {
		return fmt.Errorf("文件传输未初始化")
	}

	return qm.fileTransfer.SendMessage(targetPeer, message)
}

// GetHTTPProxy 获取HTTP代理
func (qm *QUICManager) GetHTTPProxy() *HTTPToQUICProxy {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()
	return qm.httpProxy
}

// GetP2PManager 获取P2P管理器
func (qm *QUICManager) GetP2PManager() *P2PQUICManager {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()
	return qm.p2pManager
}

// GetStatus 获取管理器状态
func (qm *QUICManager) GetStatus() map[string]interface{} {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()

	status := make(map[string]interface{})

	if qm.proxy != nil {
		status["proxy"] = qm.proxy.GetStatus()
	}

	if qm.p2pManager != nil {
		status["p2p"] = qm.p2pManager.GetConnectionInfo()
	}

	status["initialized"] = qm.isInitialized

	return status
}

// Close 关闭管理器
func (qm *QUICManager) Close() error {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()

	var errors []error

	if qm.proxy != nil {
		if err := qm.proxy.Close(); err != nil {
			errors = append(errors, fmt.Errorf("关闭QUIC代理失败: %w", err))
		}
	}

	if qm.p2pManager != nil {
		if err := qm.p2pManager.Close(); err != nil {
			errors = append(errors, fmt.Errorf("关闭P2P管理器失败: %w", err))
		}
	}

	qm.isInitialized = false

	if len(errors) > 0 {
		return fmt.Errorf("关闭管理器时发生错误: %v", errors)
	}

	log.Println("QUIC管理器已关闭")
	return nil
}

// 全局QUIC管理器实例
var GlobalQUICManager *QUICManager

// InitGlobalQUICManager 初始化全局QUIC管理器
func InitGlobalQUICManager(config *QUICConfig) error {
	manager, err := NewQUICManager(config)
	if err != nil {
		return fmt.Errorf("创建QUIC管理器失败: %w", err)
	}

	GlobalQUICManager = manager
	log.Println("全局QUIC管理器初始化成功")
	return nil
}

// GetGlobalQUICManager 获取全局QUIC管理器
func GetGlobalQUICManager() *QUICManager {
	return GlobalQUICManager
}

// InitQUICWithP2P 初始化带P2P功能的QUIC系统
func InitQUICWithP2P(localIP string, localPort int, externalIP string, externalPort int) error {
	// 初始化全局QUIC管理器
	if err := InitGlobalQUICManager(nil); err != nil {
		return err
	}

	// 初始化P2P功能
	if err := GlobalQUICManager.InitP2P(localIP, localPort, externalIP, externalPort); err != nil {
		return err
	}

	// 初始化HTTP代理
	InitHTTPToQUICProxy("localhost:8080")

	return nil
}
