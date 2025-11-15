package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
)

// WebRTCService WebRTC P2P服务
type WebRTCService struct {
	peerConnection   *webrtc.PeerConnection
	dataChannels     map[string]*webrtc.DataChannel
	channelLock      sync.RWMutex
	httpServers      map[string]*http.Server
	serverLock       sync.RWMutex
	signalingManager *SignalingManager
	done             chan struct{}
	onConnect        func(string)
}

// SignalingManager 信令管理器 - 用于交换SDP和ICE候选
type SignalingManager struct {
	localOffer   *webrtc.SessionDescription
	remoteOffer  *webrtc.SessionDescription
	localAnswer  *webrtc.SessionDescription
	remoteAnswer *webrtc.SessionDescription
	candidates   []*webrtc.ICECandidate
	lock         sync.RWMutex
}

// ICECandidateJSON ICE候选序列化格式
type ICECandidateJSON struct {
	Candidate     string  `json:"candidate"`
	SDPMLineIndex *uint16 `json:"sdpMLineIndex"`
	SDPMid        *string `json:"sdpMid"`
}

// WebRTCSignal WebRTC信令消息
type WebRTCSignal struct {
	Type      string            `json:"type"`
	SDP       string            `json:"sdp,omitempty"`
	Candidate *ICECandidateJSON `json:"candidate,omitempty"`
}

// NewWebRTCService 创建新的WebRTC服务
func NewWebRTCService() (*WebRTCService, error) {
	settingEngine := webrtc.SettingEngine{}
	settingEngine.SetNetworkTypes([]webrtc.NetworkType{
		webrtc.NetworkTypeUDP4,
		webrtc.NetworkTypeUDP6,
		webrtc.NetworkTypeTCP4,
		webrtc.NetworkTypeTCP6,
	})

	mediaEngine := &webrtc.MediaEngine{}

	api := webrtc.NewAPI(
		webrtc.WithSettingEngine(settingEngine),
		webrtc.WithMediaEngine(mediaEngine),
	)

	peerConnection, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{
					"stun:stun.l.google.com:19302",
					"stun:stun1.l.google.com:19302",
					"stun:stun2.l.google.com:19302",
					"stun:stun3.l.google.com:19302",
					"stun:stun4.l.google.com:19302",
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("创建WebRTC PeerConnection失败: %w", err)
	}

	service := &WebRTCService{
		peerConnection:   peerConnection,
		dataChannels:     make(map[string]*webrtc.DataChannel),
		httpServers:      make(map[string]*http.Server),
		signalingManager: &SignalingManager{candidates: make([]*webrtc.ICECandidate, 0)},
		done:             make(chan struct{}),
	}

	// 监听数据通道
	peerConnection.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Printf("接收到数据通道: %s", dc.Label())
		service.handleDataChannel(dc)
	})

	// 监听ICE候选
	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			log.Printf("收到ICE候选: %s", candidate.String())
			service.signalingManager.lock.Lock()
			service.signalingManager.candidates = append(service.signalingManager.candidates, candidate)
			service.signalingManager.lock.Unlock()
		}
	})

	// 监听连接状态
	peerConnection.OnConnectionStateChange(func(connectionState webrtc.PeerConnectionState) {
		log.Printf("连接状态变化: %s", connectionState.String())
		if connectionState == webrtc.PeerConnectionStateFailed {
			if err := peerConnection.Close(); err != nil {
				log.Printf("关闭PeerConnection失败: %v", err)
			}
		}
	})

	return service, nil
}

// CreateDataChannel 创建数据通道
func (ws *WebRTCService) CreateDataChannel(label string) (*webrtc.DataChannel, error) {
	ws.channelLock.Lock()
	defer ws.channelLock.Unlock()

	if _, exists := ws.dataChannels[label]; exists {
		return nil, fmt.Errorf("数据通道 %s 已存在", label)
	}

	dc, err := ws.peerConnection.CreateDataChannel(label, nil)
	if err != nil {
		return nil, fmt.Errorf("创建数据通道失败: %w", err)
	}

	ws.handleDataChannel(dc)
	return dc, nil
}

// handleDataChannel 处理数据通道
func (ws *WebRTCService) handleDataChannel(dc *webrtc.DataChannel) {
	ws.channelLock.Lock()
	ws.dataChannels[dc.Label()] = dc
	ws.channelLock.Unlock()

	dc.OnOpen(func() {
		log.Printf("数据通道已打开: %s", dc.Label())
	})

	dc.OnClose(func() {
		log.Printf("数据通道已关闭: %s", dc.Label())
		ws.channelLock.Lock()
		delete(ws.dataChannels, dc.Label())
		ws.channelLock.Unlock()

		ws.serverLock.Lock()
		if server, exists := ws.httpServers[dc.Label()]; exists {
			server.Close()
			delete(ws.httpServers, dc.Label())
		}
		ws.serverLock.Unlock()
	})

	// 启动HTTP服务
	go ws.startHTTPOverWebRTC(dc)
}

// startHTTPOverWebRTC 在数据通道上运行HTTP
func (ws *WebRTCService) startHTTPOverWebRTC(dc *webrtc.DataChannel) {
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		data := msg.Data

		// 解析HTTP请求
		reader := bufio.NewReader(bytes.NewReader(data))
		req, err := http.ReadRequest(reader)
		if err != nil {
			log.Printf("解析HTTP请求失败: %v", err)
			return
		}

		log.Printf("收到HTTP请求: %s %s", req.Method, req.URL.Path)

		// 在这里可以添加自定义HTTP处理逻辑
		// 例如文件传输、文件列表等

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString("OK")),
		}
		resp.Header.Set("Content-Type", "text/plain")
		resp.Header.Set("Content-Length", "2")

		respBytes := new(bytes.Buffer)
		resp.Write(respBytes)

		err = dc.Send(respBytes.Bytes())
		if err != nil {
			log.Printf("发送HTTP响应失败: %v", err)
		}
	})
}

// CreateOffer 创建offer
func (ws *WebRTCService) CreateOffer() (string, error) {
	offer, err := ws.peerConnection.CreateOffer(nil)
	if err != nil {
		return "", fmt.Errorf("创建Offer失败: %w", err)
	}

	err = ws.peerConnection.SetLocalDescription(offer)
	if err != nil {
		return "", fmt.Errorf("设置本地SDP失败: %w", err)
	}

	ws.signalingManager.lock.Lock()
	ws.signalingManager.localOffer = &offer
	ws.signalingManager.lock.Unlock()

	// 等待ICE候选收集完成
	gatherComplete := webrtc.GatheringCompletePromise(ws.peerConnection)
	<-gatherComplete

	sdpJSON, err := json.Marshal(WebRTCSignal{
		Type: "offer",
		SDP:  offer.SDP,
	})
	if err != nil {
		return "", fmt.Errorf("JSON编码失败: %w", err)
	}

	return string(sdpJSON), nil
}

// HandleAnswer 处理answer
func (ws *WebRTCService) HandleAnswer(answerJSON string) error {
	var signal WebRTCSignal
	err := json.Unmarshal([]byte(answerJSON), &signal)
	if err != nil {
		return fmt.Errorf("JSON解析失败: %w", err)
	}

	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  signal.SDP,
	}

	err = ws.peerConnection.SetRemoteDescription(answer)
	if err != nil {
		return fmt.Errorf("设置远程SDP失败: %w", err)
	}

	ws.signalingManager.lock.Lock()
	ws.signalingManager.remoteAnswer = &answer
	ws.signalingManager.lock.Unlock()

	return nil
}

// HandleRemoteOffer 处理远程offer
func (ws *WebRTCService) HandleRemoteOffer(offerJSON string) (string, error) {
	var signal WebRTCSignal
	err := json.Unmarshal([]byte(offerJSON), &signal)
	if err != nil {
		return "", fmt.Errorf("JSON解析失败: %w", err)
	}

	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  signal.SDP,
	}

	err = ws.peerConnection.SetRemoteDescription(offer)
	if err != nil {
		return "", fmt.Errorf("设置远程SDP失败: %w", err)
	}

	ws.signalingManager.lock.Lock()
	ws.signalingManager.remoteOffer = &offer
	ws.signalingManager.lock.Unlock()

	// 创建answer
	answer, err := ws.peerConnection.CreateAnswer(nil)
	if err != nil {
		return "", fmt.Errorf("创建Answer失败: %w", err)
	}

	err = ws.peerConnection.SetLocalDescription(answer)
	if err != nil {
		return "", fmt.Errorf("设置本地SDP失败: %w", err)
	}

	ws.signalingManager.lock.Lock()
	ws.signalingManager.localAnswer = &answer
	ws.signalingManager.lock.Unlock()

	// 等待ICE候选收集完成
	gatherComplete := webrtc.GatheringCompletePromise(ws.peerConnection)
	<-gatherComplete

	sdpJSON, err := json.Marshal(WebRTCSignal{
		Type: "answer",
		SDP:  answer.SDP,
	})
	if err != nil {
		return "", fmt.Errorf("JSON编码失败: %w", err)
	}

	return string(sdpJSON), nil
}

// AddICECandidate 添加ICE候选
func (ws *WebRTCService) AddICECandidate(candidateJSON string) error {
	var signal WebRTCSignal
	err := json.Unmarshal([]byte(candidateJSON), &signal)
	if err != nil {
		return fmt.Errorf("JSON解析失败: %w", err)
	}

	if signal.Candidate == nil {
		return fmt.Errorf("ICE候选为空")
	}

	candidate := webrtc.ICECandidateInit{
		Candidate:     signal.Candidate.Candidate,
		SDPMLineIndex: signal.Candidate.SDPMLineIndex,
		SDPMid:        signal.Candidate.SDPMid,
	}

	err = ws.peerConnection.AddICECandidate(candidate)
	if err != nil {
		return fmt.Errorf("添加ICE候选失败: %w", err)
	}

	return nil
}

// GetICECandidates 获取ICE候选
func (ws *WebRTCService) GetICECandidates() ([]string, error) {
	ws.signalingManager.lock.RLock()
	candidates := ws.signalingManager.candidates
	ws.signalingManager.lock.RUnlock()

	var result []string
	for _, candidate := range candidates {
		signalJSON, err := json.Marshal(WebRTCSignal{
			Type: "candidate",
			Candidate: &ICECandidateJSON{
				Candidate:     candidate.ToJSON().Candidate,
				SDPMLineIndex: candidate.ToJSON().SDPMLineIndex,
				SDPMid:        candidate.ToJSON().SDPMid,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("JSON编码失败: %w", err)
		}
		result = append(result, string(signalJSON))
	}

	return result, nil
}

// SendMessage 通过数据通道发送消息
func (ws *WebRTCService) SendMessage(label string, message []byte) error {
	ws.channelLock.RLock()
	dc, exists := ws.dataChannels[label]
	ws.channelLock.RUnlock()

	if !exists {
		return fmt.Errorf("数据通道 %s 不存在", label)
	}

	return dc.Send(message)
}

// SendHTTPRequest 通过WebRTC发送HTTP请求
func (ws *WebRTCService) SendHTTPRequest(label string, method, path string, body []byte) ([]byte, error) {
	ws.channelLock.RLock()
	dc, exists := ws.dataChannels[label]
	ws.channelLock.RUnlock()

	if !exists {
		return nil, fmt.Errorf("数据通道 %s 不存在", label)
	}

	req, _ := http.NewRequest(method, path, nil)
	if body != nil {
		req.ContentLength = int64(len(body))
	}

	reqBytes := new(bytes.Buffer)
	req.Write(reqBytes)

	respChan := make(chan []byte, 1)

	onMessage := func(msg webrtc.DataChannelMessage) {
		select {
		case respChan <- msg.Data:
		default:
		}
	}

	dc.OnMessage(onMessage)

	err := dc.Send(reqBytes.Bytes())
	if err != nil {
		return nil, fmt.Errorf("发送HTTP请求失败: %w", err)
	}

	select {
	case resp := <-respChan:
		return resp, nil
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("HTTP请求超时")
	}
}

// GetConnectionState 获取连接状态
func (ws *WebRTCService) GetConnectionState() string {
	return ws.peerConnection.ConnectionState().String()
}

// Close 关闭WebRTC服务
func (ws *WebRTCService) Close() error {
	close(ws.done)

	ws.channelLock.Lock()
	for _, dc := range ws.dataChannels {
		if err := dc.Close(); err != nil {
			log.Printf("关闭数据通道失败: %v", err)
		}
	}
	ws.dataChannels = make(map[string]*webrtc.DataChannel)
	ws.channelLock.Unlock()

	ws.serverLock.Lock()
	for _, server := range ws.httpServers {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		server.Shutdown(ctx)
		cancel()
	}
	ws.httpServers = make(map[string]*http.Server)
	ws.serverLock.Unlock()

	return ws.peerConnection.Close()
}

// GlobalWebRTCService 全局WebRTC服务
var GlobalWebRTCService *WebRTCService
