package transport

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

	"GoFileShare/internal/domain"

	"github.com/quic-go/quic-go"
)

type UDPTransport struct {
	listener     *net.UDPConn
	listenerDone chan struct{}
	listenPort   int
	quicPort     int
}

func NewUDPTransport(listenPort, quicPort int) *UDPTransport {
	return &UDPTransport{listenPort: listenPort, quicPort: quicPort}
}

func (t *UDPTransport) StartResponseListener(port int, key string) error {
	if t.listener != nil {
		return fmt.Errorf("监听器已在运行")
	}

	localAddr := &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: port}
	listener, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return fmt.Errorf("无法监听UDP端口 %d: %w", port, err)
	}

	t.listener = listener
	t.listenerDone = make(chan struct{})

	go t.handleReadResponses(key)
	log.Printf("P2P响应监听器已启动，监听端口: %d", port)

	return nil
}

func (t *UDPTransport) handleReadResponses(key string) {
	defer close(t.listenerDone)
	defer t.listener.Close()

	buffer := make([]byte, 1024)
	for {
		select {
		case <-t.listenerDone:
			return
		default:
		}

		t.listener.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, remoteAddr, err := t.listener.ReadFromUDP(buffer)
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
			_, err := t.listener.WriteToUDP(response, remoteAddr)
			if err != nil {
				log.Printf("发送响应失败: %v", err)
			} else {
				log.Printf("向 %s 发送响应成功", remoteAddr.String())
			}
		}
	}
}

func (t *UDPTransport) StopResponseListener() {
	if t.listener != nil {
		close(t.listenerDone)
		t.listener = nil
	}
}

func (t *UDPTransport) ConnectPeerTest(ctx context.Context, key string, info *domain.HolePunchInfo) error {
	if info == nil || !info.HasRequest {
		return fmt.Errorf("无效的打洞信息")
	}
	if err := ctx.Err(); err != nil {
		return err
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

func (t *UDPTransport) ConnectPeer(ctx context.Context, key string, info *domain.HolePunchInfo) error {
	if info == nil || !info.HasRequest {
		return fmt.Errorf("无效的打洞信息")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := t.establishP2PConnection(ctx, key, info.ExternalIP, info.ExternalPort)
	return err
}

func (t *UDPTransport) establishP2PConnection(ctx context.Context, key, externalIP, externalPort string) (*quic.Conn, error) {
	cert, err := getOrCreateCertificate()
	if err != nil {
		return nil, err
	}

	const appProtocol = "p2p-file-share"

	localAddr := &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: t.quicPort}
	udpConn, err := createReuseUDPConn(localAddr)
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
