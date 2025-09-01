package quic

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

// P2PQUICManager P2P QUIC连接管理器
type P2PQUICManager struct {
	localIP      string
	localPort    int
	externalIP   string
	externalPort int
	tlsConfig    *tls.Config
	connections  map[string]*P2PQUICConnection
	listeners    map[string]*quic.Listener
	mutex        sync.RWMutex
	isRunning    bool
}

// P2PQUICConnection P2P QUIC连接
type P2PQUICConnection struct {
	PeerKey     string
	RemoteIP    string
	RemotePort  int
	Connection  *quic.Conn
	IsConnected bool
	ConnectedAt time.Time
}

// NewP2PQUICManager 创建P2P QUIC管理器
func NewP2PQUICManager(localIP string, localPort int, externalIP string, externalPort int) (*P2PQUICManager, error) {
	tlsConfig, err := generateTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("生成TLS配置失败: %w", err)
	}

	return &P2PQUICManager{
		localIP:      localIP,
		localPort:    localPort,
		externalIP:   externalIP,
		externalPort: externalPort,
		tlsConfig:    tlsConfig,
		connections:  make(map[string]*P2PQUICConnection),
		listeners:    make(map[string]*quic.Listener),
		isRunning:    true,
	}, nil
}

// StartListener 启动QUIC监听器
func (p *P2PQUICManager) StartListener(addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("解析地址失败: %w", err)
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("监听UDP失败: %w", err)
	}

	listener, err := quic.Listen(udpConn, p.tlsConfig, &quic.Config{
		KeepAlivePeriod: 30 * time.Second,
		MaxIdleTimeout:  300 * time.Second,
	})
	if err != nil {
		udpConn.Close()
		return fmt.Errorf("创建QUIC监听器失败: %w", err)
	}

	p.mutex.Lock()
	p.listeners[addr] = listener
	p.mutex.Unlock()

	log.Printf("QUIC监听器启动在: %s", addr)

	// 启动接受连接的goroutine
	go p.acceptConnections(listener, addr)

	return nil
}

// acceptConnections 接受连接
func (p *P2PQUICManager) acceptConnections(listener *quic.Listener, addr string) {
	for p.isRunning {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			if p.isRunning {
				log.Printf("接受QUIC连接失败: %v", err)
			}
			continue
		}

		remoteAddr := conn.RemoteAddr().String()
		log.Printf("接受到新的QUIC连接: %s", remoteAddr)

		// 创建P2P连接对象
		p2pConn := &P2PQUICConnection{
			PeerKey:     remoteAddr, // 临时使用地址作为key
			RemoteIP:    conn.RemoteAddr().(*net.UDPAddr).IP.String(),
			RemotePort:  conn.RemoteAddr().(*net.UDPAddr).Port,
			Connection:  conn,
			IsConnected: true,
			ConnectedAt: time.Now(),
		}

		p.mutex.Lock()
		p.connections[remoteAddr] = p2pConn
		p.mutex.Unlock()

		// 启动连接处理
		go p.handleConnection(p2pConn)
	}
}

// handleConnection 处理连接
func (p *P2PQUICManager) handleConnection(p2pConn *P2PQUICConnection) {
	defer func() {
		p.mutex.Lock()
		delete(p.connections, p2pConn.PeerKey)
		p.mutex.Unlock()
		p2pConn.Connection.CloseWithError(0, "连接关闭")
	}()

	for {
		stream, err := p2pConn.Connection.AcceptStream(context.Background())
		if err != nil {
			log.Printf("接受流失败: %v", err)
			return
		}

		go p.handleStream(stream, p2pConn.PeerKey)
	}
}

// handleStream 处理流
func (p *P2PQUICManager) handleStream(stream *quic.Stream, peerKey string) {
	defer stream.Close()

	// 这里可以根据流的内容处理不同类型的数据
	// 例如文件传输、HTTP请求等
	buffer := make([]byte, 4096)
	n, err := stream.Read(buffer)
	if err != nil {
		log.Printf("读取流数据失败: %v", err)
		return
	}

	data := string(buffer[:n])
	log.Printf("收到来自 %s 的数据: %s", peerKey, data[:min(100, len(data))])

	// 简单回显
	stream.Write([]byte("QUIC ECHO: " + data))
}

// ConnectToPeer 连接到对等节点
func (p *P2PQUICManager) ConnectToPeer(peerKey, peerIP string, peerPort int) error {
	addr := fmt.Sprintf("%s:%d", peerIP, peerPort)

	// 检查是否已经连接
	p.mutex.RLock()
	if conn, exists := p.connections[peerKey]; exists && conn.IsConnected {
		p.mutex.RUnlock()
		return fmt.Errorf("已经连接到对等节点: %s", peerKey)
	}
	p.mutex.RUnlock()

	// 创建QUIC连接
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := quic.DialAddr(ctx, addr, &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-p2p"},
	}, &quic.Config{
		KeepAlivePeriod: 30 * time.Second,
		MaxIdleTimeout:  300 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("连接到对等节点失败: %w", err)
	}

	// 创建P2P连接对象
	p2pConn := &P2PQUICConnection{
		PeerKey:     peerKey,
		RemoteIP:    peerIP,
		RemotePort:  peerPort,
		Connection:  conn,
		IsConnected: true,
		ConnectedAt: time.Now(),
	}

	p.mutex.Lock()
	p.connections[peerKey] = p2pConn
	p.mutex.Unlock()

	log.Printf("成功连接到对等节点: %s (%s)", peerKey, addr)

	// 启动连接处理
	go p.handleConnection(p2pConn)

	return nil
}

// SendDataToPeer 发送数据到对等节点
func (p *P2PQUICManager) SendDataToPeer(peerKey string, data []byte) error {
	p.mutex.RLock()
	conn, exists := p.connections[peerKey]
	p.mutex.RUnlock()

	if !exists || !conn.IsConnected {
		return fmt.Errorf("对等节点连接不存在: %s", peerKey)
	}

	// 创建新的流
	stream, err := conn.Connection.OpenStreamSync(context.Background())
	if err != nil {
		return fmt.Errorf("打开流失败: %w", err)
	}
	defer stream.Close()

	// 发送数据
	_, err = stream.Write(data)
	if err != nil {
		return fmt.Errorf("发送数据失败: %w", err)
	}

	log.Printf("向 %s 发送了 %d 字节数据", peerKey, len(data))
	return nil
}

// GetConnection 获取指定对等节点的连接
func (p *P2PQUICManager) GetConnection(peerKey string) (*P2PQUICConnection, bool) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	conn, exists := p.connections[peerKey]
	return conn, exists
}

// IsConnectedToPeer 检查是否连接到指定对等节点
func (p *P2PQUICManager) IsConnectedToPeer(peerKey string) bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	conn, exists := p.connections[peerKey]
	if !exists {
		return false
	}

	// 检查连接是否仍然有效
	if conn.Connection == nil {
		return false
	}

	// 可以添加更多的连接健康检查
	return true
}

// GetAllConnections 获取所有连接
func (p *P2PQUICManager) GetAllConnections() map[string]*P2PQUICConnection {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	result := make(map[string]*P2PQUICConnection)
	for key, conn := range p.connections {
		result[key] = conn
	}
	return result
}

// DisconnectPeer 断开对等节点连接
func (p *P2PQUICManager) DisconnectPeer(peerKey string) error {
	p.mutex.Lock()
	conn, exists := p.connections[peerKey]
	if exists {
		delete(p.connections, peerKey)
	}
	p.mutex.Unlock()

	if !exists {
		return fmt.Errorf("对等节点连接不存在: %s", peerKey)
	}

	err := conn.Connection.CloseWithError(0, "主动断开连接")
	log.Printf("断开对等节点连接: %s", peerKey)
	return err
}

// Close 关闭管理器
func (p *P2PQUICManager) Close() error {
	p.isRunning = false

	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 关闭所有连接
	for peerKey, conn := range p.connections {
		conn.Connection.CloseWithError(0, "管理器关闭")
		log.Printf("关闭连接: %s", peerKey)
	}
	p.connections = make(map[string]*P2PQUICConnection)

	// 关闭所有监听器
	for addr, listener := range p.listeners {
		listener.Close()
		log.Printf("关闭监听器: %s", addr)
	}
	p.listeners = make(map[string]*quic.Listener)

	log.Println("P2P QUIC管理器已关闭")
	return nil
}

// GetConnectionInfo 获取连接信息
func (p *P2PQUICManager) GetConnectionInfo() map[string]interface{} {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	info := make(map[string]interface{})
	info["local_ip"] = p.localIP
	info["local_port"] = p.localPort
	info["external_ip"] = p.externalIP
	info["external_port"] = p.externalPort
	info["connection_count"] = len(p.connections)
	info["listener_count"] = len(p.listeners)

	connections := make([]map[string]interface{}, 0)
	for key, conn := range p.connections {
		connInfo := map[string]interface{}{
			"peer_key":     key,
			"remote_ip":    conn.RemoteIP,
			"remote_port":  conn.RemotePort,
			"is_connected": conn.IsConnected,
			"connected_at": conn.ConnectedAt.Format(time.RFC3339),
		}
		connections = append(connections, connInfo)
	}
	info["connections"] = connections

	return info
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 全局P2P QUIC管理器实例
var GlobalP2PQUICManager *P2PQUICManager

// InitP2PQUICManager 初始化P2P QUIC管理器
func InitP2PQUICManager(localIP string, localPort int, externalIP string, externalPort int) error {
	manager, err := NewP2PQUICManager(localIP, localPort, externalIP, externalPort)
	if err != nil {
		return err
	}

	GlobalP2PQUICManager = manager

	// 启动本地监听器
	localAddr := fmt.Sprintf("%s:%d", localIP, localPort)
	err = manager.StartListener(localAddr)
	if err != nil {
		return fmt.Errorf("启动本地监听器失败: %w", err)
	}

	// 如果外部地址不同，也启动外部监听器
	if externalIP != localIP || externalPort != localPort {
		externalAddr := fmt.Sprintf("0.0.0.0:%d", externalPort)
		err = manager.StartListener(externalAddr)
		if err != nil {
			log.Printf("启动外部监听器失败: %v", err)
		}
	}

	log.Printf("P2P QUIC管理器初始化完成")
	return nil
}

// GetGlobalP2PQUICManager 获取全局P2P QUIC管理器
func GetGlobalP2PQUICManager() *P2PQUICManager {
	return GlobalP2PQUICManager
}

// SetGlobalP2PQUICManager 设置全局P2P QUIC管理器
func SetGlobalP2PQUICManager(manager *P2PQUICManager) {
	GlobalP2PQUICManager = manager
}

// ConnectToP2PPeer 通过P2P信息连接到对等节点
func ConnectToP2PPeer(peerKey, externalIP string, externalPort int, localIP string, localPort int) error {
	if GlobalP2PQUICManager == nil {
		return fmt.Errorf("P2P QUIC管理器未初始化")
	}

	// 首先尝试连接外部地址
	err := GlobalP2PQUICManager.ConnectToPeer(peerKey, externalIP, externalPort)
	if err != nil {
		// 如果外部地址连接失败，尝试本地地址
		log.Printf("外部地址连接失败，尝试本地地址: %v", err)
		err = GlobalP2PQUICManager.ConnectToPeer(peerKey, localIP, localPort)
		if err != nil {
			return fmt.Errorf("连接到对等节点失败: %w", err)
		}
	}

	return nil
}
