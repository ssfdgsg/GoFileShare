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
	OutIP   string
	OutPort string
	Key     string
}

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
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: 8080})
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
		service := &P2PService{
			OutIP: mappedAddr.IP.String(), OutPort: strconv.Itoa(mappedAddr.Port), Key: uniqueKey,
		}
		GlobalP2PClient = service
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 生成自签名证书或使用已有证书
	cert, err := tc.getOrCreateCertificate()
	if err != nil {
		log.Printf("获取证书失败: %v", err)
		return nil
	}

	// 统一协议名称
	const appProtocol = "p2p-file-share"

	// 第一步：建立统一的UDP Socket
	localAddr := &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: 8080}
	udpConn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		log.Printf("无法创建UDP连接: %v", err)
		return nil
	}
	defer func() {
		if err := udpConn.Close(); err != nil {
			log.Printf("关闭UDP连接失败: %v", err)
		}
	}()

	// 第二步：使用UDP Socket进行NAT穿透
	remoteAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(tc.ExternalIP, tc.ExternalPort))
	if err != nil {
		log.Printf("无法解析远程地址: %v", err)
		return nil
	}

	// 发送打洞包
	punchMessage := []byte("PUNCH:" + GlobalP2PClient.Key)
	for i := 0; i < 10; i++ {
		_, err := udpConn.WriteToUDP(punchMessage, remoteAddr)
		if err != nil {
			log.Printf("发送打洞包失败 (尝试 %d): %v", i+1, err)
		} else {
			log.Printf("发送打洞包成功 (尝试 %d)", i+1)
		}
		time.Sleep(200 * time.Millisecond)
	}

	// 第三步：在同一UDP连接上建立QUIC连接
	connChan := make(chan *quic.Conn, 1)
	errChan := make(chan error, 2)

	// TLS配置
	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{appProtocol},
	}

	clientTLSConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{appProtocol},
	}

	// QUIC配置
	quicConf := &quic.Config{
		HandshakeIdleTimeout: 10 * time.Second,
		MaxIdleTimeout:       30 * time.Second,
		KeepAlivePeriod:      5 * time.Second,
	}

	// 监听协程 - 作为服务器接受连接
	go func() {
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

		session, err := listener.Accept(ctx)
		if err != nil {
			if ctx.Err() == nil {
				errChan <- fmt.Errorf("接受QUIC连接失败: %w", err)
			}
			return
		}

		log.Printf("成功接受入站QUIC连接")
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
		// 稍微延迟，给对方启动监听器的时间
		time.Sleep(1 * time.Second)

		log.Printf("尝试建立出站QUIC连接到: %s", remoteAddr.String())

		// 使用正确的QUIC Dial函数签名
		session, err := quic.Dial(ctx, udpConn, remoteAddr, clientTLSConf, quicConf)
		if err != nil {
			if ctx.Err() == nil {
				errChan <- fmt.Errorf("建立QUIC连接失败: %w", err)
			}
			return
		}

		log.Printf("成功建立出站QUIC连接")
		select {
		case connChan <- session:
		case <-ctx.Done():
			if err := session.CloseWithError(quic.ApplicationErrorCode(0), "timeout"); err != nil {
				log.Printf("关闭会话失败: %v", err)
			}
		}
	}()

	// 等待连接建立或超时
	select {
	case session := <-connChan:
		log.Println("P2P QUIC连接已成功建立！")
		return session
	case err := <-errChan:
		log.Printf("连接失败: %v", err)
		// 继续等待另一个goroutine
		select {
		case session := <-connChan:
			log.Println("P2P QUIC连接已成功建立！")
			return session
		case <-ctx.Done():
			log.Printf("连接超时: 30秒内未能建立连接")
			return nil
		}
	case <-ctx.Done():
		log.Printf("连接超时: 30秒内未能建立连接")
		return nil
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
