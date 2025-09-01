package quic

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

// QUICConfig QUIC配置
type QUICConfig struct {
	KeepAlivePeriod time.Duration
	MaxIdleTimeout  time.Duration
	MaxStreamCount  int64
	MaxDataSize     int64
}

// DefaultQUICConfig 默认QUIC配置
func DefaultQUICConfig() *QUICConfig {
	return &QUICConfig{
		KeepAlivePeriod: 30 * time.Second,
		MaxIdleTimeout:  300 * time.Second,
		MaxStreamCount:  100,
		MaxDataSize:     1024 * 1024 * 10, // 10MB
	}
}

// QUICProxy QUIC代理核心
type QUICProxy struct {
	config      *QUICConfig
	tlsConfig   *tls.Config
	listeners   map[string]*quic.Listener
	connections map[string]*quic.Conn
	mutex       sync.RWMutex
	isRunning   bool
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewQUICProxy 创建新的QUIC代理
func NewQUICProxy(config *QUICConfig) (*QUICProxy, error) {
	if config == nil {
		config = DefaultQUICConfig()
	}

	tlsConfig, err := generateTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("生成TLS配置失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &QUICProxy{
		config:      config,
		tlsConfig:   tlsConfig,
		listeners:   make(map[string]*quic.Listener),
		connections: make(map[string]*quic.Conn),
		isRunning:   true,
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

// StartListener 启动QUIC监听器
func (q *QUICProxy) StartListener(addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("解析地址失败: %w", err)
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("监听UDP失败: %w", err)
	}

	quicConfig := &quic.Config{
		KeepAlivePeriod: q.config.KeepAlivePeriod,
		MaxIdleTimeout:  q.config.MaxIdleTimeout,
	}

	listener, err := quic.Listen(udpConn, q.tlsConfig, quicConfig)
	if err != nil {
		udpConn.Close()
		return fmt.Errorf("创建QUIC监听器失败: %w", err)
	}

	q.mutex.Lock()
	q.listeners[addr] = listener
	q.mutex.Unlock()

	log.Printf("QUIC监听器启动在: %s", addr)

	// 启动接受连接的协程
	go q.acceptConnections(listener, addr)

	return nil
}

// acceptConnections 接受连接
func (q *QUICProxy) acceptConnections(listener *quic.Listener, addr string) {
	for q.isRunning {
		select {
		case <-q.ctx.Done():
			return
		default:
			conn, err := listener.Accept(q.ctx)
			if err != nil {
				if q.isRunning {
					log.Printf("接受QUIC连接失败: %v", err)
				}
				continue
			}

			remoteAddr := conn.RemoteAddr().String()
			log.Printf("新的QUIC连接: %s", remoteAddr)

			q.mutex.Lock()
			q.connections[remoteAddr] = conn
			q.mutex.Unlock()

			// 启动连接处理协程
			go q.handleConnection(conn, remoteAddr)
		}
	}
}

// handleConnection 处理连接
func (q *QUICProxy) handleConnection(conn *quic.Conn, addr string) {
	defer func() {
		q.mutex.Lock()
		delete(q.connections, addr)
		q.mutex.Unlock()
		conn.CloseWithError(0, "连接处理完成")
		log.Printf("连接已关闭: %s", addr)
	}()

	for {
		select {
		case <-q.ctx.Done():
			return
		case <-conn.Context().Done():
			return
		default:
			stream, err := conn.AcceptStream(q.ctx)
			if err != nil {
				log.Printf("接受流失败 [%s]: %v", addr, err)
				return
			}

			// 为每个流启动处理协程
			go q.handleStream(stream, addr)
		}
	}
}

// handleStream 处理流
func (q *QUICProxy) handleStream(stream *quic.Stream, addr string) {
	defer func() {
		if err := stream.Close(); err != nil {
			log.Printf("关闭流失败 [%s]: %v", addr, err)
		}
	}()

	// 设置流读取超时
	ctx, cancel := context.WithTimeout(q.ctx, 30*time.Second)
	defer cancel()

	// 读取流数据
	buffer := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			stream.SetReadDeadline(time.Now().Add(10 * time.Second))
			n, err := stream.Read(buffer)
			if err != nil {
				if err != io.EOF {
					log.Printf("读取流数据失败 [%s]: %v", addr, err)
				}
				return
			}

			if n > 0 {
				data := buffer[:n]
				log.Printf("收到数据 [%s]: %d bytes", addr, n)

				// 处理接收到的数据
				if err := q.processStreamData(stream, data, addr); err != nil {
					log.Printf("处理流数据失败 [%s]: %v", addr, err)
					return
				}
			}
		}
	}
}

// processStreamData 处理流数据
func (q *QUICProxy) processStreamData(stream *quic.Stream, data []byte, addr string) error {
	// 简单的回显处理，实际应用中可以根据协议进行不同处理
	response := fmt.Sprintf("ECHO[%s]: %s", time.Now().Format("15:04:05"), string(data))

	stream.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err := stream.Write([]byte(response))
	if err != nil {
		return fmt.Errorf("写入响应失败: %w", err)
	}

	return nil
}

// Connect 连接到远程地址
func (q *QUICProxy) Connect(addr string) (*quic.Conn, error) {
	q.mutex.RLock()
	if conn, exists := q.connections[addr]; exists {
		q.mutex.RUnlock()
		return conn, nil
	}
	q.mutex.RUnlock()

	ctx, cancel := context.WithTimeout(q.ctx, 10*time.Second)
	defer cancel()

	quicConfig := &quic.Config{
		KeepAlivePeriod: q.config.KeepAlivePeriod,
		MaxIdleTimeout:  q.config.MaxIdleTimeout,
	}

	conn, err := quic.DialAddr(ctx, addr, &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-proxy"},
	}, quicConfig)
	if err != nil {
		return nil, fmt.Errorf("连接失败: %w", err)
	}

	q.mutex.Lock()
	q.connections[addr] = conn
	q.mutex.Unlock()

	log.Printf("成功连接到: %s", addr)

	// 启动连接处理
	go q.handleConnection(conn, addr)

	return conn, nil
}

// SendData 发送数据到指定地址
func (q *QUICProxy) SendData(addr string, data []byte) error {
	conn, err := q.Connect(addr)
	if err != nil {
		return fmt.Errorf("获取连接失败: %w", err)
	}

	ctx, cancel := context.WithTimeout(q.ctx, 30*time.Second)
	defer cancel()

	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return fmt.Errorf("打开流失败: %w", err)
	}
	defer stream.Close()

	stream.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err = stream.Write(data)
	if err != nil {
		return fmt.Errorf("发送数据失败: %w", err)
	}

	log.Printf("发送数据到 %s: %d bytes", addr, len(data))
	return nil
}

// GetConnections 获取所有连接
func (q *QUICProxy) GetConnections() map[string]*quic.Conn {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	result := make(map[string]*quic.Conn)
	for addr, conn := range q.connections {
		result[addr] = conn
	}
	return result
}

// GetConnection 获取指定连接
func (q *QUICProxy) GetConnection(addr string) (*quic.Conn, bool) {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	conn, exists := q.connections[addr]
	return conn, exists
}

// CloseConnection 关闭指定连接
func (q *QUICProxy) CloseConnection(addr string) error {
	q.mutex.Lock()
	conn, exists := q.connections[addr]
	if exists {
		delete(q.connections, addr)
	}
	q.mutex.Unlock()

	if !exists {
		return fmt.Errorf("连接不存在: %s", addr)
	}

	err := conn.CloseWithError(0, "主动关闭")
	log.Printf("关闭连接: %s", addr)
	return err
}

// Close 关闭代理
func (q *QUICProxy) Close() error {
	q.isRunning = false
	q.cancel()

	q.mutex.Lock()
	defer q.mutex.Unlock()

	// 关闭所有连接
	for addr, conn := range q.connections {
		conn.CloseWithError(0, "代理关闭")
		log.Printf("关闭连接: %s", addr)
	}
	q.connections = make(map[string]*quic.Conn)

	// 关闭所有监听器
	for addr, listener := range q.listeners {
		listener.Close()
		log.Printf("关闭监听器: %s", addr)
	}
	q.listeners = make(map[string]*quic.Listener)

	log.Println("QUIC代理已关闭")
	return nil
}

// GetStatus 获取代理状态
func (q *QUICProxy) GetStatus() map[string]interface{} {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	return map[string]interface{}{
		"is_running":       q.isRunning,
		"connection_count": len(q.connections),
		"listener_count":   len(q.listeners),
		"config":           q.config,
	}
}

// generateTLSConfig 生成TLS配置
func generateTLSConfig() (*tls.Config, error) {
	// 生成私钥
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("生成私钥失败: %w", err)
	}

	// 创建证书模板
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"GoFileShare"},
			Country:       []string{"CN"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:    []string{"localhost"},
	}

	// 生成证书
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("生成证书失败: %w", err)
	}

	// 编码私钥
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	// 编码证书
	certPEM := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	}

	// 创建TLS证书
	cert, err := tls.X509KeyPair(pem.EncodeToMemory(certPEM), pem.EncodeToMemory(privateKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("创建TLS证书失败: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"quic-proxy"},
	}, nil
}

// 全局QUIC代理实例
var GlobalQUICProxy *QUICProxy

// InitQUICProxy 初始化全局QUIC代理
func InitQUICProxy(config *QUICConfig) error {
	proxy, err := NewQUICProxy(config)
	if err != nil {
		return fmt.Errorf("创建QUIC代理失败: %w", err)
	}

	GlobalQUICProxy = proxy
	log.Println("全局QUIC代理初始化成功")
	return nil
}

// GetGlobalQUICProxy 获取全局QUIC代理
func GetGlobalQUICProxy() *QUICProxy {
	return GlobalQUICProxy
}
