package services

import (
	"GoFileShare/models"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/denisbrodbeck/machineid"
	"github.com/pion/stun"
	"github.com/quic-go/quic-go"
)

type RegisterData struct {
	Task      int8   `json:"task"`       // 1:注册 2:查询 3:打洞请求 4:打洞响应
	IP        string `json:"ip"`         // 客户端声明的IP
	Port      int    `json:"port"`       // 客户端声明的端口
	Key       string `json:"key"`        // 客户端标识
	TargetKey string `json:"target_key"` // 目标客户端标识(用于打洞)
	Timestamp int64  `json:"timestamp"`  // 时间戳
}
type P2PService struct {
	OutIP     string
	OutPort   string
	Key       string
	LocalPort int          // 本地实际使用的端口
	UDPConn   *net.UDPConn // 复用的UDP连接
}

// 全局UDP连接管理
var GlobalUDPConn *net.UDPConn

type TargetClient struct {
	ExternalIP   string // 目标客户端外网IP
	ExternalPort string // 目标客户端外网端口
}

var GlobalP2PClient *P2PService

// Register 向P2P服务器注册自己的信息
func (p *P2PService) Register() error {
	// 检查P2PService是否为空
	if p == nil {
		return fmt.Errorf("P2PService未初始化")
	}

	// 检查环境变量
	serverIP := os.Getenv("P2P_SERVER_IP")
	serverPort := os.Getenv("P2P_SERVER_PORT")

	if serverIP == "" {
		serverIP = "127.0.0.1" // 默认值
	}
	if serverPort == "" {
		serverPort = "8888" // 默认值
	}

	// 将字符串端口转换为整数
	portInt, err := strconv.Atoi(p.OutPort)
	if err != nil {
		return fmt.Errorf("端口转换失败: %w", err)
	}

	packet := RegisterData{
		Task:      1,
		IP:        p.OutIP,
		Port:      portInt,
		Key:       p.Key,
		Timestamp: time.Now().Unix(),
	}

	jsonData, err := json.Marshal(packet)
	if err != nil {
		return fmt.Errorf("JSON编码失败: %w", err)
	}

	serverURL := "http://" + serverIP + ":" + serverPort + "/api/register"
	fmt.Printf("正在向服务器注册: %s\n", serverURL) // 调试信息

	resp, err := http.Post(serverURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("发送注册请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("服务器返回错误状态码: %d", resp.StatusCode)
	}

	return nil
}

// InitInfo 使用STUN协议获取NAT类型、外网IP和端口
func InitInfo() (*P2PService, error) {
	StunList := []string{
		"stun.l.google.com:19302",
		"stun1.l.google.com:19302",
		"stun2.l.google.com:19302",
		"stun3.l.google.com:19302",
		"stun4.l.google.com:19302",
		"stun.chat.bilibili.com:3478",
		"turn.cloudflare.com:3478",
		"stun.miwifi.com:3478",
	}
	// 使用随机端口避免冲突
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: 0})
	if err != nil {
		return nil, fmt.Errorf("创建UDP连接失败: %w", err)
	}
	defer conn.Close()

	var lastErr error
	for _, stunServer := range StunList {
		serverAddr, err := net.ResolveUDPAddr("udp", stunServer)
		if err != nil {
			lastErr = fmt.Errorf("解析STUN服务器%s地址失败: %w", stunServer, err)
			continue
		}

		message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
		_, err = conn.WriteTo(message.Raw, serverAddr)
		if err != nil {
			lastErr = fmt.Errorf("发送STUN请求到%s失败: %w", stunServer, err)
			continue
		}

		buf := make([]byte, 1500)
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			lastErr = fmt.Errorf("读取STUN响应失败: %w", err)
			continue
		}

		res := new(stun.Message)
		res.Raw = buf[:n]
		if err := res.Decode(); err != nil {
			lastErr = fmt.Errorf("解码STUN响应失败: %w", err)
			continue
		}

		var mappedAddr stun.XORMappedAddress
		if err := mappedAddr.GetFrom(res); err != nil {
			lastErr = fmt.Errorf("获取映射地址失败: %w", err)
			continue
		}
		//生成key
		machineID, err := machineid.ID()
		if err != nil {
			return nil, fmt.Errorf("无法获取本机的Machine ID: %w", err)
		}
		macs := ""
		ifaces, err := net.Interfaces()
		if err == nil {
			for _, iface := range ifaces {
				macs += iface.HardwareAddr.String()
			}
		}
		keySource := machineID + macs + mappedAddr.IP.String() + strconv.Itoa(mappedAddr.Port)
		hash := sha256.Sum256([]byte(keySource))
		uniqueKey := hex.EncodeToString(hash[:])
		// 获取本地实际端口
		localAddr := conn.LocalAddr().(*net.UDPAddr)

		service := &P2PService{
			OutIP:     mappedAddr.IP.String(),
			OutPort:   strconv.Itoa(mappedAddr.Port),
			Key:       uniqueKey,
			LocalPort: localAddr.Port,
			UDPConn:   conn, // 保持连接复用
		}

		// 保存全局UDP连接
		GlobalUDPConn = conn
		GlobalP2PClient = service

		log.Printf("P2P服务初始化成功: 外网=%s:%s, 本地端口=%d",
			service.OutIP, service.OutPort, service.LocalPort)

		// 不要关闭连接，让它保持开启用于后续复用
		return service, nil
	}

	return nil, fmt.Errorf("所有STUN服务器均失败: %w", lastErr)
}

// GetHolePunch 向P2P服务器请求打洞信息
func (p *P2PService) GetHolePunch(targetKey string) (*models.HolePunchInfo, error) {
	// 检查环境变量
	serverIP := os.Getenv("P2P_SERVER_IP")
	serverPort := os.Getenv("P2P_SERVER_PORT")

	if serverIP == "" {
		serverIP = "127.0.0.1" // 默认值
	}
	if serverPort == "" {
		serverPort = "8888" // 默认值
	}

	serverURL := "http://" + serverIP + ":" + serverPort + "/api/get-hole-punch?client_key=" + targetKey

	resp, err := http.Get(serverURL)
	if err != nil {
		return nil, fmt.Errorf("请求打洞信息失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务器返回错误状态码: %d", resp.StatusCode)
	}

	var apiResponse models.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("解析响应JSON失败: %w", err)
	}

	if apiResponse.Status != "success" {
		return nil, fmt.Errorf("API返回错误: %s", apiResponse.Message)
	}

	// 解析响应数据
	data, ok := apiResponse.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("响应数据格式错误")
	}

	holePunchInfo := &models.HolePunchInfo{}

	// 检查是否有请求
	if hasRequest, ok := data["has_request"].(bool); ok {
		holePunchInfo.HasRequest = hasRequest
	}

	// 如果没有请求，直接返回
	if !holePunchInfo.HasRequest {
		return holePunchInfo, nil
	}

	// 解析打洞请求详细信息
	if requesterKey, ok := data["requester_key"].(string); ok {
		holePunchInfo.RequesterKey = requesterKey
	}
	if externalIP, ok := data["external_ip"].(string); ok {
		holePunchInfo.ExternalIP = externalIP
	}
	if externalPort, ok := data["external_port"].(string); ok {
		holePunchInfo.ExternalPort = externalPort
	}
	if localIP, ok := data["local_ip"].(string); ok {
		holePunchInfo.LocalIP = localIP
	}
	if localPort, ok := data["local_port"].(string); ok {
		holePunchInfo.LocalPort = localPort
	}
	if timestamp, ok := data["timestamp"].(float64); ok {
		holePunchInfo.Timestamp = int64(timestamp)
	}

	return holePunchInfo, nil
}

// ConnectPeerTest 尝试与目标客户端建立P2P连接
func (p *P2PService) ConnectPeerTest(holePunchInfo *models.HolePunchInfo) error {
	if holePunchInfo == nil || !holePunchInfo.HasRequest {
		return fmt.Errorf("无效的打洞信息")
	}
	conn, err := net.Dial("UDP", holePunchInfo.ExternalIP+":"+holePunchInfo.ExternalPort)
	if err != nil {
		return fmt.Errorf("连接目标客户端失败: %w", err)
	}
	defer conn.Close()
	message := []byte("P2P Connection Test from " + p.Key)
	_, err = conn.Write(message)
	if err != nil {
		return err
	}
	buffer := make([]byte, 1024)
	_, err = conn.Read(buffer)
	if err != nil {
		return err
	}

	fmt.Println("Received:", string(buffer))
	return nil
}

func (tc *TargetClient) EchoPeer() {
	qConn := tc.establishP2PConnection()
	if qConn == nil {
		log.Println("无法建立P2P连接")
		return
	}
	defer func() {
		if err := qConn.CloseWithError(quic.ApplicationErrorCode(0), "closing"); err != nil {
			log.Printf("关闭QUIC连接失败: %v", err)
		}
	}()

	log.Printf("P2P连接建立成功！开始处理数据传输...")

	// 创建上下文用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动连接处理
	tc.handleP2PConnection(ctx, qConn)
}

// handleP2PConnection 处理P2P连接的数据传输
func (tc *TargetClient) handleP2PConnection(ctx context.Context, conn *quic.Conn) {
	log.Printf("开始处理P2P连接...")

	// 启动两个goroutine分别处理收发
	go tc.handleIncomingStreams(ctx, conn)
	go tc.sendHeartbeat(ctx, conn)

	// 保持连接活跃
	<-ctx.Done()
	log.Printf("P2P连接处理结束")
}

// handleIncomingStreams 处理入站数据流
func (tc *TargetClient) handleIncomingStreams(ctx context.Context, conn *quic.Conn) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		stream, err := conn.AcceptStream(ctx)
		if err != nil {
			log.Printf("接受流失败: %v", err)
			return
		}

		// 为每个流启动处理goroutine
		go tc.handleStream(ctx, stream)
	}
}

// handleStream 处理单个数据流
func (tc *TargetClient) handleStream(ctx context.Context, stream quic.Stream) {
	defer stream.Close()

	log.Printf("接收到新的数据流: %d", stream.StreamID())

	// 读取流数据
	buffer := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := stream.Read(buffer)
		if err != nil {
			if err.Error() != "EOF" {
				log.Printf("读取流数据失败: %v", err)
			}
			return
		}

		message := string(buffer[:n])
		log.Printf("收到P2P消息: %s", message)

		// 处理不同类型的消息
		tc.processMessage(ctx, stream, message)
	}
}

// processMessage 处理接收到的消息
func (tc *TargetClient) processMessage(ctx context.Context, stream quic.Stream, message string) {
	if message == "PING" {
		// 响应心跳
		_, err := stream.Write([]byte("PONG"))
		if err != nil {
			log.Printf("发送PONG失败: %v", err)
		}
	} else if message == "HELLO" {
		// 响应握手
		response := "HELLO_RESPONSE:" + GlobalP2PClient.Key
		_, err := stream.Write([]byte(response))
		if err != nil {
			log.Printf("发送握手响应失败: %v", err)
		}
	} else {
		// 处理其他消息
		log.Printf("处理消息: %s", message)
		// 这里可以添加文件传输、聊天等功能
	}
}

// sendHeartbeat 发送心跳保持连接
func (tc *TargetClient) sendHeartbeat(ctx context.Context, conn *quic.Conn) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stream, err := conn.OpenStreamSync(ctx)
			if err != nil {
				log.Printf("打开心跳流失败: %v", err)
				return
			}

			_, err = stream.Write([]byte("PING"))
			if err != nil {
				log.Printf("发送心跳失败: %v", err)
			}
			stream.Close()
		}
	}
}

func (tc *TargetClient) setupUDPSocketAndPunch(p P2PService) (net.PacketConn, net.Addr, error) {
	localAddr := fmt.Sprintf("0.0.0.0:8080")
	conn, err := net.ListenPacket("udp", localAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("无法绑定本地UDP端口: %w", err)
	}
	remoteAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(tc.ExternalIP, tc.ExternalPort))
	if err != nil {
		log.Fatalf("无法解析远程地址: %v", err)
	}
	for i := 0; i < 5; i++ {
		if _, err := conn.WriteTo([]byte("Punching from "+p.Key), remoteAddr); err != nil {
			log.Printf("第%v次发送打洞数据失败: %v", i, err)
		} else {
			log.Printf("第%v次发送打洞数据成功", i)
			break
		}
		time.Sleep(1 * time.Second)
	}
	return conn, remoteAddr, nil
}

// establishP2PConnection 使用统一UDP Socket建立P2P QUIC连接
func (tc *TargetClient) establishP2PConnection() *quic.Conn {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 生成自签名证书或使用已有证书
	cert, err := tc.getOrCreateCertificate()
	if err != nil {
		log.Printf("获取证书失败: %v", err)
		return nil
	}

	// 统一协议名称
	const appProtocol = "p2p-file-share"

	// 第一步：复用全局P2P客户端的端口信息
	if GlobalP2PClient == nil {
		log.Printf("全局P2P客户端未初始化")
		return nil
	}

	// 解析本地端口
	localPort, err := strconv.Atoi(GlobalP2PClient.OutPort)
	if err != nil {
		log.Printf("解析本地端口失败: %v", err)
		return nil
	}

	// 使用与STUN相同的端口建立UDP连接
	localAddr := &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: localPort}
	udpConn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		log.Printf("无法创建UDP连接在端口 %d: %v", localPort, err)
		// 如果端口被占用，尝试使用随机端口
		localAddr = &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: 0}
		udpConn, err = net.ListenUDP("udp", localAddr)
		if err != nil {
			log.Printf("无法创建UDP连接: %v", err)
			return nil
		}
	}
	defer func() {
		if err := udpConn.Close(); err != nil {
			log.Printf("关闭UDP连接失败: %v", err)
		}
	}()

	log.Printf("本地UDP连接已建立: %s", udpConn.LocalAddr().String())

	// 第二步：解析远程地址并进行双向NAT穿透
	remoteAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(tc.ExternalIP, tc.ExternalPort))
	if err != nil {
		log.Printf("无法解析远程地址: %v", err)
		return nil
	}

	log.Printf("开始NAT穿透到: %s", remoteAddr.String())

	// 持续发送打洞包，实现双向打洞
	punchDone := make(chan bool, 1)
	go func() {
		punchMessage := []byte("PUNCH:" + GlobalP2PClient.Key)
		for i := 0; i < 30; i++ { // 增加打洞尝试次数
			select {
			case <-ctx.Done():
				return
			case <-punchDone:
				return
			default:
			}

			_, err := udpConn.WriteToUDP(punchMessage, remoteAddr)
			if err != nil {
				log.Printf("发送打洞包失败 (尝试 %d): %v", i+1, err)
			} else {
				log.Printf("发送打洞包成功 (尝试 %d)", i+1)
			}
			time.Sleep(500 * time.Millisecond) // 增加间隔
		}
	}()

	// 监听打洞响应
	go func() {
		buffer := make([]byte, 1024)
		udpConn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, addr, err := udpConn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("读取打洞响应失败: %v", err)
			return
		}
		log.Printf("收到来自 %s 的打洞响应: %s", addr.String(), string(buffer[:n]))
		punchDone <- true
	}()

	// 等待一段时间让打洞完成
	time.Sleep(3 * time.Second)

	// 第三步：在同一UDP连接上建立QUIC连接
	connChan := make(chan *quic.Conn, 1)
	errChan := make(chan error, 2)

	// TLS配置 - 服务器端
	tlsConf := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		NextProtos:         []string{appProtocol},
		InsecureSkipVerify: true, // 允许自签名证书
	}

	// TLS配置 - 客户端
	clientTLSConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{appProtocol},
		ServerName:         "p2p-server", // 设置服务器名称
	}

	// QUIC配置 - 更宽松的超时设置
	quicConf := &quic.Config{
		HandshakeIdleTimeout:    30 * time.Second, // 增加握手超时
		MaxIdleTimeout:          60 * time.Second, // 增加最大空闲超时
		KeepAlivePeriod:         10 * time.Second, // 增加keepalive间隔
		DisablePathMTUDiscovery: true,             // 禁用路径MTU发现，避免某些网络问题
	}

	// 监听协程 - 作为服务器接受连接
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("监听协程panic: %v", r)
				errChan <- fmt.Errorf("监听协程发生panic: %v", r)
			}
		}()

		log.Printf("启动QUIC监听器，等待入站连接...")

		// 使用现有的UDP连接创建QUIC监听器
		listener, err := quic.Listen(udpConn, tlsConf, quicConf)
		if err != nil {
			errChan <- fmt.Errorf("创建QUIC监听器失败: %w", err)
			return
		}
		defer func() {
			if err := listener.Close(); err != nil {
				log.Printf("关闭监听器失败: %v", err)
			}
		}()

		log.Printf("QUIC监听器已启动，本地地址: %s", udpConn.LocalAddr().String())

		session, err := listener.Accept(ctx)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("接受QUIC连接失败: %v", err)
				errChan <- fmt.Errorf("接受QUIC连接失败: %w", err)
			}
			return
		}

		log.Printf("成功接受入站QUIC连接，远程地址: %s", session.RemoteAddr().String())
		select {
		case connChan <- session:
		case <-ctx.Done():
			if err := session.CloseWithError(quic.ApplicationErrorCode(0), "timeout"); err != nil {
				log.Printf("关闭会话失败: %v", err)
			}
		}
	}()

	// 拨号协程 - 作为客户端发起连接
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("拨号协程panic: %v", r)
				errChan <- fmt.Errorf("拨号协程发生panic: %v", r)
			}
		}()

		// 等待NAT穿透完成和对方监听器启动
		time.Sleep(5 * time.Second)

		log.Printf("尝试建立出站QUIC连接到: %s", remoteAddr.String())

		// 多次重试连接
		var lastErr error
		for i := 0; i < 3; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}

			log.Printf("第 %d 次尝试连接...", i+1)

			// 使用正确的QUIC Dial函数签名
			session, err := quic.Dial(ctx, udpConn, remoteAddr, clientTLSConf, quicConf)
			if err != nil {
				lastErr = err
				log.Printf("第 %d 次连接失败: %v", i+1, err)
				time.Sleep(2 * time.Second)
				continue
			}

			log.Printf("成功建立出站QUIC连接，本地地址: %s, 远程地址: %s",
				session.LocalAddr().String(), session.RemoteAddr().String())
			select {
			case connChan <- session:
				return
			case <-ctx.Done():
				if err := session.CloseWithError(quic.ApplicationErrorCode(0), "timeout"); err != nil {
					log.Printf("关闭会话失败: %v", err)
				}
				return
			}
		}

		if ctx.Err() == nil && lastErr != nil {
			errChan <- fmt.Errorf("所有连接尝试均失败，最后错误: %w", lastErr)
		}
	}()

	// 等待连接建立或超时
	log.Printf("等待P2P连接建立...")
	for {
		select {
		case session := <-connChan:
			log.Printf("P2P QUIC连接已成功建立！连接信息: 本地=%s, 远程=%s",
				session.LocalAddr().String(), session.RemoteAddr().String())
			close(punchDone) // 停止打洞
			return session
		case err := <-errChan:
			log.Printf("连接过程中出现错误: %v", err)
			// 不立即返回，继续等待其他goroutine
		case <-ctx.Done():
			log.Printf("连接超时: 60秒内未能建立P2P连接")
			close(punchDone) // 停止打洞
			return nil
		}
	}
}

// 获取或创建证书的辅助方法
func (tc *TargetClient) getOrCreateCertificate() (tls.Certificate, error) {
	// 先尝试加载现有证书
	cert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
	if err == nil {
		return cert, nil
	}

	// 如果加载失败，生成临时自签名证书
	return tc.generateSelfSignedCert()
}

// 生成临时自签名证书
func (tc *TargetClient) generateSelfSignedCert() (tls.Certificate, error) {
	// 生成RSA密钥对
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("生成RSA密钥对失败: %w", err)
	}

	// 生成自签名证书
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // 证书有效期：1年

	serialNumber, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("生成序列号失败: %w", err)
	}

	certTemplate := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"P2P File Share"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &certTemplate, &certTemplate, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("创建自签名证书失败: %w", err)
	}

	// 保存私钥和证书到文件
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

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
	}, nil
}
