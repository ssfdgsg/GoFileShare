package models

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	"github.com/pion/stun"
	"github.com/quic-go/quic-go"
)

// P2PInfo P2P信息
type P2PInfo struct {
	OutIP   string `json:"out_ip"`
	OutPort string `json:"out_port"`
	Key     string `json:"key"`
}

// HolePunchInfo 打洞信息
type HolePunchInfo struct {
	ExternalIP   string `json:"external_ip"`
	ExternalPort string `json:"external_port"`
	HasRequest   bool   `json:"has_request"`
}

// P2PManager P2P管理器
type P2PManager struct {
	info         *P2PInfo
	listener     *net.UDPConn
	listenerDone chan struct{}
	listenPort   int
	quicPort     int
	stunServers  []string
	registeredAt int64
}

var p2pManager *P2PManager

// InitP2PManager 初始化P2P管理器
func InitP2PManager(listenPort, quicPort int, stunServers []string) {
	p2pManager = &P2PManager{
		listenPort:  listenPort,
		quicPort:    quicPort,
		stunServers: stunServers,
	}
}

// GetP2PManager 获取P2P管理器实例
func GetP2PManager() *P2PManager {
	return p2pManager
}

// DiscoverP2PInfo 发现P2P信息（通过STUN）
func (m *P2PManager) DiscoverP2PInfo(ctx context.Context) (*P2PInfo, error) {
	var lastErr error
	for _, server := range m.stunServers {
		conn, err := net.Dial("udp", server)
		if err != nil {
			lastErr = err
			continue
		}
		defer conn.Close()

		message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
		if _, err := conn.Write(message.Raw); err != nil {
			lastErr = err
			continue
		}

		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			lastErr = err
			continue
		}

		var response stun.Message
		response.Raw = buf[:n]
		if err := response.Decode(); err != nil {
			lastErr = err
			continue
		}

		var xorAddr stun.XORMappedAddress
		if err := xorAddr.GetFrom(&response); err != nil {
			lastErr = err
			continue
		}

		info := &P2PInfo{
			OutIP:   xorAddr.IP.String(),
			OutPort: fmt.Sprintf("%d", xorAddr.Port),
		}
		
		if m.info != nil && m.info.Key != "" {
			info.Key = m.info.Key
		}
		
		m.info = info
		return info, nil
	}

	return nil, fmt.Errorf("所有STUN服务器均失败: %v", lastErr)
}

// SetP2PKey 设置P2P密钥
func (m *P2PManager) SetP2PKey(key string) {
	if key == "" {
		return
	}
	if m.info == nil {
		m.info = &P2PInfo{}
	}
	m.info.Key = key
}

// GetP2PInfo 获取P2P信息
func (m *P2PManager) GetP2PInfo() *P2PInfo {
	return m.info
}

// StartResponseListener 启动响应监听器
func (m *P2PManager) StartResponseListener(port int, key string) error {
	if m.listener != nil {
		return fmt.Errorf("监听器已在运行")
	}

	localAddr := &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: port}
	listener, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return fmt.Errorf("无法监听UDP端口 %d: %w", port, err)
	}

	m.listener = listener
	m.listenerDone = make(chan struct{})
	m.registeredAt = time.Now().Unix()

	go m.handleReadResponses(key)
	log.Printf("P2P响应监听器已启动，监听端口: %d", port)

	return nil
}

// handleReadResponses 处理读取响应
func (m *P2PManager) handleReadResponses(key string) {
	defer close(m.listenerDone)
	defer m.listener.Close()

	buffer := make([]byte, 1024)
	for {
		select {
		case <-m.listenerDone:
			return
		default:
		}

		m.listener.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, remoteAddr, err := m.listener.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			log.Printf("读取UDP数据失败: %v", err)
			continue
		}

		data := string(buffer[:n])
		log.Printf("收到来自 %s 的消息: %s", remoteAddr.String(), data)

		if strings.Contains(data, "P2P Connection Test") {
			response := []byte("P2P Connection Response from " + key)
			_, err := m.listener.WriteToUDP(response, remoteAddr)
			if err != nil {
				log.Printf("发送响应失败: %v", err)
			} else {
				log.Printf("向 %s 发送响应成功", remoteAddr.String())
			}
		}
	}
}

// ConnectPeerTest 测试P2P连接
func (m *P2PManager) ConnectPeerTest(ctx context.Context, key string, info *HolePunchInfo) error {
	if info == nil || !info.HasRequest {
		return fmt.Errorf("无效的打洞信息")
	}

	localAddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	if err != nil {
		return fmt.Errorf("解析本地地址失败: %w", err)
	}

	listener, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return fmt.Errorf("创建本地UDP监听器失败: %w", err)
	}
	defer listener.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = listener.SetReadDeadline(deadline)
	}

	remoteAddr, err := net.ResolveUDPAddr("udp", info.ExternalIP+":"+info.ExternalPort)
	if err != nil {
		return fmt.Errorf("解析远程地址失败: %w", err)
	}

	respChan := make(chan []byte, 1)
	errChan := make(chan error, 1)

	go func() {
		buffer := make([]byte, 1024)
		n, _, err := listener.ReadFromUDP(buffer)
		if err != nil {
			errChan <- err
			return
		}
		respChan <- buffer[:n]
	}()

	message := []byte("P2P Connection Test from " + key)
	_, err = listener.WriteToUDP(message, remoteAddr)
	if err != nil {
		return fmt.Errorf("发送打洞消息失败: %w", err)
	}

	select {
	case resp := <-respChan:
		log.Println("Received:", string(resp))
		return nil
	case err := <-errChan:
		return fmt.Errorf("读取响应失败: %w", err)
	case <-ctx.Done():
		return fmt.Errorf("等待响应超时")
	}
}

// ConnectPeer 连接到对等节点
func (m *P2PManager) ConnectPeer(ctx context.Context, key string, info *HolePunchInfo) error {
	if info == nil || !info.HasRequest {
		return fmt.Errorf("无效的打洞信息")
	}
	_, err := m.establishP2PConnection(ctx, key, info.ExternalIP, info.ExternalPort)
	return err
}

// establishP2PConnection 建立P2P连接
func (m *P2PManager) establishP2PConnection(ctx context.Context, key, externalIP, externalPort string) (*quic.Conn, error) {
	cert, err := getOrCreateCertificate()
	if err != nil {
		return nil, err
	}

	const appProtocol = "p2p-file-share"

	localAddr := &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: m.quicPort}
	udpConn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return nil, err
	}

	remoteAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(externalIP, externalPort))
	if err != nil {
		return nil, fmt.Errorf("无法解析远程地址: %v", err)
	}

	punchCtx, punchCancel := context.WithCancel(ctx)
	defer punchCancel()

	punchMessage := []byte("PUNCH:" + key)
	go continuousPunching(punchCtx, udpConn, remoteAddr, punchMessage)

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("连接超时")
	case <-time.After(500 * time.Millisecond):
	}

	connChan := make(chan *quic.Conn, 1)
	errChan := make(chan error, 2)

	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{appProtocol},
	}

	clientTLSConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{appProtocol},
	}

	quicConf := &quic.Config{
		HandshakeIdleTimeout: 30 * time.Second,
		MaxIdleTimeout:       60 * time.Second,
		KeepAlivePeriod:      10 * time.Second,
	}

	go func() {
		listener, err := quic.Listen(udpConn, tlsConf, quicConf)
		if err != nil {
			errChan <- fmt.Errorf("创建QUIC监听器失败: %w", err)
			return
		}
		defer listener.Close()

		session, err := listener.Accept(ctx)
		if err != nil {
			if ctx.Err() == nil {
				errChan <- fmt.Errorf("接受QUIC连接失败: %w", err)
			}
			return
		}

		select {
		case connChan <- session:
		case <-ctx.Done():
			session.CloseWithError(quic.ApplicationErrorCode(0), "timeout")
		}
	}()

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(200 * time.Millisecond):
		}
		session, err := quic.Dial(ctx, udpConn, remoteAddr, clientTLSConf, quicConf)
		if err != nil {
			if ctx.Err() == nil {
				errChan <- fmt.Errorf("建立QUIC连接失败: %w", err)
			}
			return
		}

		select {
		case connChan <- session:
		case <-ctx.Done():
			session.CloseWithError(quic.ApplicationErrorCode(0), "timeout")
		}
	}()

	select {
	case session := <-connChan:
		return session, nil
	case err := <-errChan:
		select {
		case session := <-connChan:
			return session, nil
		case <-ctx.Done():
			return nil, err
		}
	case <-ctx.Done():
		return nil, fmt.Errorf("连接超时")
	}
}

// GetRegisteredAt 获取注册时间
func (m *P2PManager) GetRegisteredAt() int64 {
	return m.registeredAt
}

// 辅助函数
func continuousPunching(ctx context.Context, conn *net.UDPConn, remoteAddr *net.UDPAddr, message []byte) {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	attempt := 0
	for {
		select {
		case <-ctx.Done():
			log.Printf("停止NAT打洞（总共发送 %d 次）", attempt)
			return
		case <-ticker.C:
			attempt++
			_, err := conn.WriteToUDP(message, remoteAddr)
			if err != nil {
				log.Printf("发送打洞包失败 (尝试 %d): %v", attempt, err)
			} else if attempt%5 == 1 {
				log.Printf("持续发送打洞包 (尝试 %d)", attempt)
			}
		}
	}
}

func getOrCreateCertificate() (tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
	if err == nil {
		return cert, nil
	}
	return generateSelfSignedCert()
}

func generateSelfSignedCert() (tls.Certificate, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("生成RSA密钥对失败: %w", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumber, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("生成序列号失败: %w", err)
	}

	certTemplate := x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{Organization: []string{"P2P File Share"}},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &certTemplate, &certTemplate, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("创建自签名证书失败: %w", err)
	}

	privBytes := x509.MarshalPKCS1PrivateKey(priv)
	privFile, err := os.OpenFile("key.pem", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("无法打开私钥文件保存私钥: %w", err)
	}
	defer privFile.Close()

	if err := pem.Encode(privFile, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return tls.Certificate{}, fmt.Errorf("无法写入私钥文件: %w", err)
	}

	certFile, err := os.OpenFile("cert.pem", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("无法打开证书文件保存证书: %w", err)
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return tls.Certificate{}, fmt.Errorf("无法写入证书文件: %w", err)
	}

	return tls.Certificate{Certificate: [][]byte{certDER}, PrivateKey: priv}, nil
}
