package services

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/pion/webrtc/v4"
)

// SignalingMessage 信令消息结构
type SignalingMessage struct {
	MessageID string          `json:"message_id"`
	Type      string          `json:"type"`
	From      string          `json:"from"`
	To        string          `json:"to"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp int64           `json:"timestamp"`
}

// SignalingServer WebRTC信令服务器
type SignalingServer struct {
	connections map[string]*SignalingConnection
	mutex       sync.RWMutex
	messageQ    chan SignalingMessage
	done        chan struct{}
}

// SignalingConnection WebRTC信令连接
type SignalingConnection struct {
	ClientID      string
	ExternalIP    string
	ExternalPort  string
	LastOffer     *webrtc.SessionDescription
	LastAnswer    *webrtc.SessionDescription
	ICECandidates []*webrtc.ICECandidate
	Connected     bool
	LastHeartbeat int64
}

// OfferPayload Offer消息载荷
type OfferPayload struct {
	SDP string `json:"sdp"`
}

// AnswerPayload Answer消息载荷
type AnswerPayload struct {
	SDP string `json:"sdp"`
}

// CandidatePayload 候选消息载荷
type CandidatePayload struct {
	Candidate     string  `json:"candidate"`
	SDPMLineIndex *uint16 `json:"sdpMLineIndex"`
	SDPMid        *string `json:"sdpMid"`
}

// NewSignalingServer 创建新的信令服务器
func NewSignalingServer() *SignalingServer {
	return &SignalingServer{
		connections: make(map[string]*SignalingConnection),
		messageQ:    make(chan SignalingMessage, 100),
		done:        make(chan struct{}),
	}
}

// RegisterClient 注册客户端
func (ss *SignalingServer) RegisterClient(clientID, externalIP, externalPort string) error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	if _, exists := ss.connections[clientID]; exists {
		return fmt.Errorf("客户端 %s 已注册", clientID)
	}

	ss.connections[clientID] = &SignalingConnection{
		ClientID:      clientID,
		ExternalIP:    externalIP,
		ExternalPort:  externalPort,
		Connected:     true,
		ICECandidates: make([]*webrtc.ICECandidate, 0),
	}

	log.Printf("客户端已注册: %s (%s:%s)", clientID, externalIP, externalPort)
	return nil
}

// UnregisterClient 注销客户端
func (ss *SignalingServer) UnregisterClient(clientID string) error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	if _, exists := ss.connections[clientID]; !exists {
		return fmt.Errorf("客户端 %s 未注册", clientID)
	}

	delete(ss.connections, clientID)
	log.Printf("客户端已注销: %s", clientID)
	return nil
}

// GetClientInfo 获取客户端信息
func (ss *SignalingServer) GetClientInfo(clientID string) (*SignalingConnection, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	conn, exists := ss.connections[clientID]
	if !exists {
		return nil, fmt.Errorf("客户端 %s 未找到", clientID)
	}

	return conn, nil
}

// SendMessage 发送信令消息
func (ss *SignalingServer) SendMessage(msg SignalingMessage) error {
	ss.mutex.RLock()
	fromConn, fromExists := ss.connections[msg.From]
	toConn, toExists := ss.connections[msg.To]
	ss.mutex.RUnlock()

	if !fromExists {
		return fmt.Errorf("发送者 %s 未找到", msg.From)
	}

	if !toExists {
		return fmt.Errorf("接收者 %s 未找到", msg.To)
	}

	if !fromConn.Connected || !toConn.Connected {
		return fmt.Errorf("连接状态异常")
	}

	select {
	case ss.messageQ <- msg:
		log.Printf("消息已发送: %s -> %s (类型: %s)", msg.From, msg.To, msg.Type)
		return nil
	case <-ss.done:
		return fmt.Errorf("信令服务器已关闭")
	default:
		return fmt.Errorf("消息队列已满")
	}
}

// GetMessage 获取消息
func (ss *SignalingServer) GetMessage() (SignalingMessage, error) {
	select {
	case msg := <-ss.messageQ:
		return msg, nil
	case <-ss.done:
		return SignalingMessage{}, fmt.Errorf("信令服务器已关闭")
	}
}

// StoreOffer 存储Offer
func (ss *SignalingServer) StoreOffer(clientID string, offer *webrtc.SessionDescription) error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	conn, exists := ss.connections[clientID]
	if !exists {
		return fmt.Errorf("客户端 %s 未找到", clientID)
	}

	conn.LastOffer = offer
	return nil
}

// GetOffer 获取Offer
func (ss *SignalingServer) GetOffer(clientID string) (*webrtc.SessionDescription, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	conn, exists := ss.connections[clientID]
	if !exists {
		return nil, fmt.Errorf("客户端 %s 未找到", clientID)
	}

	return conn.LastOffer, nil
}

// StoreAnswer 存储Answer
func (ss *SignalingServer) StoreAnswer(clientID string, answer *webrtc.SessionDescription) error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	conn, exists := ss.connections[clientID]
	if !exists {
		return fmt.Errorf("客户端 %s 未找到", clientID)
	}

	conn.LastAnswer = answer
	return nil
}

// GetAnswer 获取Answer
func (ss *SignalingServer) GetAnswer(clientID string) (*webrtc.SessionDescription, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	conn, exists := ss.connections[clientID]
	if !exists {
		return nil, fmt.Errorf("客户端 %s 未找到", clientID)
	}

	return conn.LastAnswer, nil
}

// AddICECandidate 添加ICE候选
func (ss *SignalingServer) AddICECandidate(clientID string, candidate *webrtc.ICECandidate) error {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	conn, exists := ss.connections[clientID]
	if !exists {
		return fmt.Errorf("客户端 %s 未找到", clientID)
	}

	conn.ICECandidates = append(conn.ICECandidates, candidate)
	log.Printf("添加ICE候选到 %s (总计: %d)", clientID, len(conn.ICECandidates))
	return nil
}

// GetICECandidates 获取ICE候选
func (ss *SignalingServer) GetICECandidates(clientID string) ([]*webrtc.ICECandidate, error) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	conn, exists := ss.connections[clientID]
	if !exists {
		return nil, fmt.Errorf("客户端 %s 未找到", clientID)
	}

	return conn.ICECandidates, nil
}

// Close 关闭信令服务器
func (ss *SignalingServer) Close() {
	close(ss.done)
	ss.mutex.Lock()
	ss.connections = make(map[string]*SignalingConnection)
	ss.mutex.Unlock()
}

// GlobalSignalingServer 全局信令服务器
var GlobalSignalingServer *SignalingServer

// InitSignalingServer 初始化全局信令服务器
func InitSignalingServer() {
	if GlobalSignalingServer == nil {
		GlobalSignalingServer = NewSignalingServer()
	}
}
