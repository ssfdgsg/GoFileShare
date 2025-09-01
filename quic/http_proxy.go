package quic

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/quic-go/quic-go"
)

// HTTPToQUICProxy HTTP到QUIC的代理
type HTTPToQUICProxy struct {
	quicProxy  *QUICProxy
	targetAddr string
	timeout    time.Duration
}

// NewHTTPToQUICProxy 创建HTTP到QUIC代理
func NewHTTPToQUICProxy(targetAddr string) *HTTPToQUICProxy {
	return &HTTPToQUICProxy{
		targetAddr: targetAddr,
		timeout:    30 * time.Second,
	}
}

// SetQUICProxy 设置QUIC代理
func (h *HTTPToQUICProxy) SetQUICProxy(proxy *QUICProxy) {
	h.quicProxy = proxy
}

// ProxyHandler Gin中间件处理HTTP代理
func (h *HTTPToQUICProxy) ProxyHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否需要通过QUIC代理
		if h.shouldProxy(c) {
			h.handleQUICProxy(c)
			return
		}

		// 继续正常处理
		c.Next()
	}
}

// shouldProxy 判断是否需要代理
func (h *HTTPToQUICProxy) shouldProxy(c *gin.Context) bool {
	// 检查请求头中是否包含QUIC代理标识
	if c.GetHeader("X-QUIC-Proxy") == "true" {
		return true
	}

	// 检查是否是P2P相关的请求
	if strings.HasPrefix(c.Request.URL.Path, "/api/p2p/") {
		return true
	}

	return false
}

// handleQUICProxy 处理QUIC代理请求
func (h *HTTPToQUICProxy) handleQUICProxy(c *gin.Context) {
	if h.quicProxy == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "QUIC代理未初始化",
		})
		return
	}

	// 读取请求体
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "读取请求体失败",
		})
		return
	}

	// 创建新的请求体读取器
	bodyReader := bytes.NewReader(bodyBytes)

	// 通过QUIC代理发送请求
	resp, err := h.sendViaQUIC(c.Request.Method, c.Request.URL.Path, bodyReader)
	if err != nil {
		log.Printf("QUIC代理请求失败: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "QUIC代理请求失败",
			"details": err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	// 复制响应头
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// 设置状态码
	c.Status(resp.StatusCode)

	// 复制响应体
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		log.Printf("复制响应体失败: %v", err)
	}
}

// sendViaQUIC 通过QUIC发送请求
func (h *HTTPToQUICProxy) sendViaQUIC(method, path string, body io.Reader) (*http.Response, error) {
	// 获取QUIC连接
	connections := h.quicProxy.GetConnections()
	if len(connections) == 0 {
		return nil, fmt.Errorf("没有可用的QUIC连接")
	}

	// 使用第一个可用连接
	var conn *quic.Conn
	for _, c := range connections {
		conn = c
		break
	}

	// 创建流
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("打开QUIC流失败: %w", err)
	}
	defer stream.Close()

	// 构造简化的HTTP请求
	request := fmt.Sprintf("%s %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n",
		method, path, h.targetAddr)

	// 发送请求头
	if _, err := stream.Write([]byte(request)); err != nil {
		return nil, fmt.Errorf("发送请求头失败: %w", err)
	}

	// 发送请求体
	if body != nil {
		if _, err := io.Copy(stream, body); err != nil {
			return nil, fmt.Errorf("发送请求体失败: %w", err)
		}
	}

	// 读取响应
	response := &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Body:       io.NopCloser(stream),
	}

	return response, nil
}

// QUICFileTransfer QUIC文件传输
type QUICFileTransfer struct {
	proxy   *QUICProxy
	timeout time.Duration
}

// NewQUICFileTransfer 创建QUIC文件传输
func NewQUICFileTransfer(proxy *QUICProxy) *QUICFileTransfer {
	return &QUICFileTransfer{
		proxy:   proxy,
		timeout: 300 * time.Second, // 5分钟超时
	}
}

// SendFile 发送文件
func (q *QUICFileTransfer) SendFile(targetPeer, filePath string, fileData io.Reader, fileSize int64) error {
	connections := q.proxy.GetConnections()
	conn, exists := connections[targetPeer]
	if !exists {
		return fmt.Errorf("目标对等节点连接不存在: %s", targetPeer)
	}

	// 创建文件传输流
	ctx, cancel := context.WithTimeout(context.Background(), q.timeout)
	defer cancel()

	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return fmt.Errorf("打开文件传输流失败: %w", err)
	}
	defer stream.Close()

	// 发送文件头信息
	header := fmt.Sprintf("FILE_TRANSFER:%s:%d\n", filePath, fileSize)
	if _, err := stream.Write([]byte(header)); err != nil {
		return fmt.Errorf("发送文件头失败: %w", err)
	}

	// 发送文件数据
	written, err := io.Copy(stream, fileData)
	if err != nil {
		return fmt.Errorf("发送文件数据失败: %w", err)
	}

	log.Printf("文件发送完成: %s -> %s (%d bytes)", filePath, targetPeer, written)
	return nil
}

// ReceiveFile 接收文件
func (q *QUICFileTransfer) ReceiveFile(stream *quic.Stream) (string, int64, io.Reader, error) {
	// 读取文件头
	headerBuffer := make([]byte, 1024)
	n, err := stream.Read(headerBuffer)
	if err != nil {
		return "", 0, nil, fmt.Errorf("读取文件头失败: %w", err)
	}

	headerStr := string(headerBuffer[:n])
	if !strings.HasPrefix(headerStr, "FILE_TRANSFER:") {
		return "", 0, nil, fmt.Errorf("无效的文件传输头: %s", headerStr)
	}

	// 解析文件头: FILE_TRANSFER:filename:size
	parts := strings.SplitN(headerStr, ":", 3)
	if len(parts) < 3 {
		return "", 0, nil, fmt.Errorf("文件头格式错误: %s", headerStr)
	}

	filePath := strings.TrimSpace(parts[1])
	fileSize := int64(0)
	if _, err := fmt.Sscanf(parts[2], "%d", &fileSize); err != nil {
		return "", 0, nil, fmt.Errorf("解析文件大小失败: %w", err)
	}

	log.Printf("开始接收文件: %s (%d bytes)", filePath, fileSize)

	// 返回文件信息和数据流
	return filePath, fileSize, stream, nil
}

// SendMessage 发送简单消息
func (q *QUICFileTransfer) SendMessage(targetPeer, message string) error {
	connections := q.proxy.GetConnections()
	conn, exists := connections[targetPeer]
	if !exists {
		return fmt.Errorf("目标对等节点连接不存在: %s", targetPeer)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return fmt.Errorf("打开消息流失败: %w", err)
	}
	defer stream.Close()

	// 发送消息头和内容
	msg := fmt.Sprintf("MESSAGE:%s\n", message)
	if _, err := stream.Write([]byte(msg)); err != nil {
		return fmt.Errorf("发送消息失败: %w", err)
	}

	log.Printf("消息发送到 %s: %s", targetPeer, message)
	return nil
}

// 全局HTTP到QUIC代理实例
var GlobalHTTPToQUICProxy *HTTPToQUICProxy

// InitHTTPToQUICProxy 初始化HTTP到QUIC代理
func InitHTTPToQUICProxy(targetAddr string) {
	GlobalHTTPToQUICProxy = NewHTTPToQUICProxy(targetAddr)
}

// GetGlobalHTTPToQUICProxy 获取全局HTTP到QUIC代理
func GetGlobalHTTPToQUICProxy() *HTTPToQUICProxy {
	return GlobalHTTPToQUICProxy
}
