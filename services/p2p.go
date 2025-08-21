package services

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

// 扩展的数据包结构
type PacketData struct {
	Task      int8   `json:"task"`
	IP        string `json:"ip"`
	Port      string `json:"port"`
	Key       string `json:"key"`
	TargetKey string `json:"target_key"`
	Timestamp int64  `json:"timestamp"`
}

// 客户端信息响应
type ClientInfoResponse struct {
	Status       string `json:"status"`
	ExternalIP   string `json:"external_ip"`
	ExternalPort int    `json:"external_port"`
	LocalIP      string `json:"local_ip"`
	LocalPort    int    `json:"local_port"`
	NATType      string `json:"nat_type"`
}

// P2P连接信息
type P2PConnection struct {
	RemoteKey          string
	RemoteExternalIP   string
	RemoteExternalPort int
	RemoteLocalIP      string
	RemoteLocalPort    int
	Conn               *net.UDPConn
	IsConnected        bool
}

type EnhancedUdpClient struct {
	conn           *net.UDPConn
	serverAddr     *net.UDPAddr
	localIP        string
	localPort      int
	clientKey      string
	externalIP     string
	externalPort   int
	natType        string
	p2pConnections map[string]*P2PConnection
	mutex          sync.RWMutex
	isRunning      bool
}

func NewEnhancedUdpClient(serverAddr, clientKey string) (*EnhancedUdpClient, error) {
	sAddr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		return nil, fmt.Errorf("无法解析服务器地址: %w", err)
	}

	// 监听本地UDP端口
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: 0})
	if err != nil {
		return nil, fmt.Errorf("无法监听本地UDP端口: %w", err)
	}

	// 获取本地监听信息
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	localIP := getLocalIP()

	client := &EnhancedUdpClient{
		conn:           conn,
		serverAddr:     sAddr,
		localIP:        localIP,
		localPort:      localAddr.Port,
		clientKey:      clientKey,
		p2pConnections: make(map[string]*P2PConnection),
		isRunning:      true,
	}

	// 启动消息接收goroutine
	go client.messageReceiver()

	log.Printf("增强版客户端已启动: 本地地址=%s:%d, Key=%s", localIP, localAddr.Port, clientKey)
	return client, nil
}

// 获取本地IP地址
func getLocalIP() string {
	conn, err := net.Dial("udp", os.Getenv("baidu.com"))
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

func detectNATType() (string, net.UDPAddr) {
	localAddr, _ := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	remoteAddr, _ := net.ResolveUDPAddr("udp", "stun.l.google.com:19302")

	conn, _ := net.DialUDP("udp", localAddr, remoteAddr)
	defer conn.Close()

	// 发送STUN请求
	_, _ = conn.Write([]byte("STUN Request"))

	// 设置超时
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	buffer := make([]byte, 1024)
	n, _, _ := conn.ReadFromUDP(buffer)

	// 解析STUN响应
	if n > 0 {
		// 简化处理，实际需解析STUN响应格式
		return "Full Cone NAT", *remoteAddr
	}
	return "Unknown NAT Type", *remoteAddr
}

// Register 注册到信令服务器并获取NAT信息
func (c *EnhancedUdpClient) Register() error {
	NATType, ADDR := detectNATType()
	if NATType == "Full Cone NAT" {
		c.natType = NATType
	} else {
		c.natType = "Restricted Cone NAT"
	}
	packet := PacketData{
		Task:      1,
		IP:        ADDR.IP.String(),
		Port:      strconv.Itoa(ADDR.Port),
		Key:       c.clientKey,
		Timestamp: time.Now().Unix(),
	}

	jsonData, err := json.Marshal(packet)
	if err != nil {
		return fmt.Errorf("JSON编码失败: %w", err)
	}

	_, err = c.conn.WriteToUDP(jsonData, c.serverAddr)
	if err != nil {
		return fmt.Errorf("发送注册请求失败: %w", err)
	}

	// 等待响应
	c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buffer := make([]byte, 1024)
	n, _, err := c.conn.ReadFromUDP(buffer)
	if err != nil {
		return fmt.Errorf("等待注册响应失败: %w", err)
	}

	var response ClientInfoResponse
	err = json.Unmarshal(buffer[:n], &response)
	if err != nil {
		return fmt.Errorf("解析注册响应失败: %w", err)
	}

	if response.Status == "registered" {
		c.externalIP = response.ExternalIP
		c.externalPort = response.ExternalPort
		c.natType = response.NATType
		log.Printf("注册成功: 外部地址=%s:%d, NAT类型=%s", c.externalIP, c.externalPort, c.natType)
		return nil
	}

	return fmt.Errorf("注册失败: %s", response.Status)
}

// ConnectToPeer 连接到目标客户端
func (c *EnhancedUdpClient) ConnectToPeer(targetKey string) error {
	// 1. 查询目标客户端信息
	targetInfo, err := c.queryPeerInfo(targetKey)
	if err != nil {
		return fmt.Errorf("查询目标客户端失败: %w", err)
	}

	// 2. 请求NAT打洞
	err = c.requestHolePunch(targetKey)
	if err != nil {
		return fmt.Errorf("请求打洞失败: %w", err)
	}

	// 3. 尝试建立P2P连接
	err = c.establishP2PConnection(targetKey, targetInfo)
	if err != nil {
		return fmt.Errorf("建立P2P连接失败: %w", err)
	}

	return nil
}

func (c *EnhancedUdpClient) queryPeerInfo(targetKey string) (*ClientInfoResponse, error) {
	packet := PacketData{
		Task:      2,
		IP:        c.localIP,
		Port:      strconv.Itoa(c.localPort),
		Key:       c.clientKey,
		TargetKey: targetKey,
		Timestamp: time.Now().Unix(),
	}

	jsonData, _ := json.Marshal(packet)
	c.conn.WriteToUDP(jsonData, c.serverAddr)

	c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buffer := make([]byte, 1024)
	n, _, err := c.conn.ReadFromUDP(buffer)
	if err != nil {
		return nil, err
	}

	var response ClientInfoResponse
	err = json.Unmarshal(buffer[:n], &response)
	if err != nil {
		return nil, err
	}

	if response.Status == "found" || response.Status == "found_in_redis" {
		return &response, nil
	}

	return nil, fmt.Errorf("目标客户端不存在: %s", response.Status)
}

func (c *EnhancedUdpClient) requestHolePunch(targetKey string) error {
	packet := PacketData{
		Task:      3,
		IP:        c.localIP,
		Port:      strconv.Itoa(c.localPort),
		Key:       c.clientKey,
		TargetKey: targetKey,
		Timestamp: time.Now().Unix(),
	}

	jsonData, _ := json.Marshal(packet)
	c.conn.WriteToUDP(jsonData, c.serverAddr)

	c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buffer := make([]byte, 1024)
	n, _, err := c.conn.ReadFromUDP(buffer)
	if err != nil {
		return err
	}

	var response map[string]interface{}
	json.Unmarshal(buffer[:n], &response)

	if response["status"] == "hole_punch_initiated" {
		log.Printf("打洞请求已发送给目标客户端: %s", targetKey)
		return nil
	}

	return fmt.Errorf("打洞请求失败: %v", response["status"])
}

func (c *EnhancedUdpClient) establishP2PConnection(targetKey string, targetInfo *ClientInfoResponse) error {
	// 实现UDP打洞逻辑
	// 1. 同时向目标的外部地址和本地地址发送UDP包
	// 2. 建立双向通信通道

	externalAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", targetInfo.ExternalIP, targetInfo.ExternalPort))
	localAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", targetInfo.LocalIP, targetInfo.LocalPort))

	// 发送打洞包到外部地址
	punchMsg := []byte(fmt.Sprintf("PUNCH_FROM_%s", c.clientKey))
	c.conn.WriteToUDP(punchMsg, externalAddr)

	// 如果在同一内网，也尝试本地地址
	if c.isInSameNetwork(targetInfo.LocalIP) {
		c.conn.WriteToUDP(punchMsg, localAddr)
	}

	// 等待连接确认（简化实现）
	time.Sleep(2 * time.Second)

	// 创建P2P连接对象
	p2pConn := &P2PConnection{
		RemoteKey:          targetKey,
		RemoteExternalIP:   targetInfo.ExternalIP,
		RemoteExternalPort: targetInfo.ExternalPort,
		RemoteLocalIP:      targetInfo.LocalIP,
		RemoteLocalPort:    targetInfo.LocalPort,
		Conn:               c.conn,
		IsConnected:        true,
	}

	c.mutex.Lock()
	c.p2pConnections[targetKey] = p2pConn
	c.mutex.Unlock()

	log.Printf("P2P连接建立成功: %s", targetKey)
	return nil
}

func (c *EnhancedUdpClient) isInSameNetwork(remoteIP string) bool {
	// 简化的同网络检测
	if len(c.localIP) < 7 || len(remoteIP) < 7 {
		return false
	}
	return c.localIP[:7] == remoteIP[:7] // 比较前三个网段
}

// 消息接收器
func (c *EnhancedUdpClient) messageReceiver() {
	buffer := make([]byte, 1024)
	for c.isRunning {
		c.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, err := c.conn.ReadFrom(buffer)
		if err != nil {
			continue
		}

		// 检查是否是打洞通知
		var notification map[string]interface{}
		if json.Unmarshal(buffer[:n], &notification) == nil {
			if task, ok := notification["task"].(float64); ok && int(task) == 4 {
				log.Printf("收到打洞通知，来自: %v", notification["requester_key"])
				// 响应打洞请求
				c.respondToHolePunch(addr, notification)
			}
		} else {
			// 处理P2P数据
			log.Printf("收到P2P数据: %s", string(buffer[:n]))
		}
	}
}

func (c *EnhancedUdpClient) respondToHolePunch(addr net.Addr, notification map[string]interface{}) {
	// 向请求者发送响应
	response := []byte(fmt.Sprintf("PUNCH_RESPONSE_FROM_%s", c.clientKey))
	udpAddr, _ := net.ResolveUDPAddr("udp", addr.String())
	c.conn.WriteToUDP(response, udpAddr)
}

// SendP2PMessage 发送P2P消息
func (c *EnhancedUdpClient) SendP2PMessage(targetKey, message string) error {
	c.mutex.RLock()
	conn, exists := c.p2pConnections[targetKey]
	c.mutex.RUnlock()

	if !exists || !conn.IsConnected {
		return fmt.Errorf("与 %s 的P2P连接不存在", targetKey)
	}

	// 尝试多个地址发送（提高成功率）
	externalAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", conn.RemoteExternalIP, conn.RemoteExternalPort))
	localAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", conn.RemoteLocalIP, conn.RemoteLocalPort))

	msgData := []byte(message)
	c.conn.WriteToUDP(msgData, externalAddr)
	if c.isInSameNetwork(conn.RemoteLocalIP) {
		c.conn.WriteToUDP(msgData, localAddr)
	}

	return nil
}

// GetClientInfo 获取客户端状态信息
func (c *EnhancedUdpClient) GetClientInfo() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return map[string]interface{}{
		"client_key":    c.clientKey,
		"local_ip":      c.localIP,
		"local_port":    c.localPort,
		"external_ip":   c.externalIP,
		"external_port": c.externalPort,
		"nat_type":      c.natType,
		"is_running":    c.isRunning,
		"connections":   len(c.p2pConnections),
	}
}

// GetP2PConnections 获取P2P连接列表
func (c *EnhancedUdpClient) GetP2PConnections() map[string]*P2PConnection {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// 返回连接的副本以避免并发问题
	connections := make(map[string]*P2PConnection)
	for k, v := range c.p2pConnections {
		connections[k] = v
	}
	return connections
}

func (c *EnhancedUdpClient) Close() {
	c.isRunning = false
	if c.conn != nil {
		c.conn.Close()
	}
}

// GlobalEnhancedP2PClient 全局客户端实例
var GlobalEnhancedP2PClient *EnhancedUdpClient

// InitEnhancedP2PClient 初始化增强版P2P客户端
func InitEnhancedP2PClient(serverAddr, clientKey string) error {
	client, err := NewEnhancedUdpClient(serverAddr, clientKey)
	if err != nil {
		return err
	}
	GlobalEnhancedP2PClient = client
	return nil
}

// GetGlobalEnhancedP2PClient 获取全局增强版P2P客户端
func GetGlobalEnhancedP2PClient() *EnhancedUdpClient {
	return GlobalEnhancedP2PClient
}

// InitP2PClient 初始化P2P客户端（向后兼容）
func InitP2PClient(serverAddr string) error {
	clientKey := fmt.Sprintf("fileserver-%d", time.Now().Unix())
	return InitEnhancedP2PClient(serverAddr, clientKey)
}

// GetGlobalP2PClient 获取全局P2P客户端（向后兼容）
func GetGlobalP2PClient() *EnhancedUdpClient {
	return GetGlobalEnhancedP2PClient()
}
