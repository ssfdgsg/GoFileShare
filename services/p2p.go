package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/donnie4w/go-logger/logger"
	"log"
	"net"
	"time"
)

// PacketData 结构体必须与服务端保持一致
type PacketData struct {
	Task int8   `json:"Task"`
	IP   string `json:"ip"`
	Key  string `json:"key"`
}

// UdpClient 封装了客户端的操作
type UdpClient struct {
	conn       *net.UDPConn
	serverAddr *net.UDPAddr
	localIP    string
}

// NewUdpClient 创建一个新的UDP客户端实例
func NewUdpClient(serverAddr string) (*UdpClient, error) {
	// 解析服务器地址
	sAddr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		return nil, fmt.Errorf("无法解析服务器地址: %w", err)
	}

	// 监听一个随机的本地UDP端口用于通信
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: 0})
	if err != nil {
		return nil, fmt.Errorf("无法监听本地UDP端口: %w", err)
	}

	// 获取本地出站IP，用于填充PacketData中的IP字段
	localIP, err := getOutboundIP()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("无法获取本地IP: %w", err)
	}

	log.Printf("客户端已在 %s 启动，将连接到 %s", conn.LocalAddr().String(), sAddr.String())

	return &UdpClient{
		conn:       conn,
		serverAddr: sAddr,
		localIP:    localIP,
	}, nil
}

// Register 向服务器注册自己的Key
// key: 你希望注册的唯一标识符
func (c *UdpClient) Register(key string) error {
	log.Printf("正在注册 Key: %s...", key)

	// 1. 准备数据包
	packet := PacketData{
		Task: 1,
		IP:   c.localIP, // 发送自己的IP用于服务器验证
		Key:  key,
	}
	jsonData, err := json.Marshal(packet)
	if err != nil {
		return fmt.Errorf("JSON编码失败: %w", err)
	}

	// 2. 发送数据
	_, err = c.conn.WriteToUDP(jsonData, c.serverAddr)
	if err != nil {
		return fmt.Errorf("发送注册请求失败: %w", err)
	}

	// 根据修复后的服务端逻辑，可以等待一个确认响应
	// 设置5秒的读取超时
	err = c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		return err
	}
	buffer := make([]byte, 1024)
	n, _, err := c.conn.ReadFromUDP(buffer)
	if err != nil {
		return fmt.Errorf("等待注册响应失败: %w", err)
	}

	response := string(buffer[:n])
	log.Printf("收到注册响应: %s", response)
	if response != "Registered" {
		return fmt.Errorf("注册失败，服务器响应: %s", response)
	}

	return nil
}

// GetIPByKey 通过一个Key从服务器获取对应的IP地址
// keyToLookup: 你希望查询的那个Key
func (c *UdpClient) GetIPByKey(keyToLookup string) (string, error) {
	log.Printf("正在查询 Key: %s...", keyToLookup)

	// 1. 准备数据包
	packet := PacketData{
		Task: 2,
		IP:   c.localIP, // 发送自己的IP用于服务器验证
		Key:  keyToLookup,
	}
	jsonData, err := json.Marshal(packet)
	if err != nil {
		return "", fmt.Errorf("JSON编码失败: %w", err)
	}

	// 2. 发送数据
	_, err = c.conn.WriteToUDP(jsonData, c.serverAddr)
	if err != nil {
		return "", fmt.Errorf("发送查询请求失败: %w", err)
	}

	// 3. 等待服务器响应
	// 设置5秒的读取超时
	err = c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		return "", err
	}
	buffer := make([]byte, 1024)
	n, _, err := c.conn.ReadFromUDP(buffer)
	if err != nil {
		return "", fmt.Errorf("等待查询响应失败: %w", err)
	}

	response := string(buffer[:n])
	log.Printf("收到查询响应: %s", response)

	if response == "Not found" {
		return "", errors.New("服务器未找到对应的客户端")
	}

	return response, nil
}

// Close 关闭客户端连接
func (c *UdpClient) Close() {
	err := c.conn.Close()
	if err != nil {
		return
	}
}

// getOutboundIP 获取本机的首选出站IP地址
func getOutboundIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			logger.Error("Error while closing connection %v", err)
		}
	}(conn)

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}
