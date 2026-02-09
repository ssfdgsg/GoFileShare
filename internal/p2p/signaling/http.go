package signaling

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"GoFileShare/internal/domain"
)

type RegisterData struct {
	Task      int8   `json:"task"`
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	Key       string `json:"key"`
	TargetKey string `json:"target_key"`
	Timestamp int64  `json:"timestamp"`
}

type apiResponseEnvelope struct {
	Status  string          `json:"status"`
	Message string          `json:"message,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type HTTPClient struct {
	serverIP   string
	serverPort string
	client     *http.Client
}

func NewHTTPClient(serverIP, serverPort string) *HTTPClient {
	return &HTTPClient{
		serverIP:   serverIP,
		serverPort: serverPort,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *HTTPClient) Register(ctx context.Context, info domain.P2PInfo) error {
	portInt, err := strconv.Atoi(info.OutPort)
	if err != nil {
		return fmt.Errorf("端口转换失败: %w", err)
	}

	packet := RegisterData{
		Task:      1,
		IP:        info.OutIP,
		Port:      portInt,
		Key:       info.Key,
		Timestamp: time.Now().Unix(),
	}

	jsonData, err := json.Marshal(packet)
	if err != nil {
		return fmt.Errorf("JSON编码失败: %w", err)
	}

	serverURL := "http://" + c.serverIP + ":" + c.serverPort + "/api/register"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serverURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建注册请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送注册请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("服务器返回错误状态码: %d", resp.StatusCode)
	}

	return nil
}

func (c *HTTPClient) GetHolePunch(ctx context.Context, targetKey string) (*domain.HolePunchInfo, error) {
	serverURL := "http://" + c.serverIP + ":" + c.serverPort + "/api/get-hole-punch?client_key=" + targetKey

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求打洞信息失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务器返回错误状态码: %d", resp.StatusCode)
	}

	var apiResponse apiResponseEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("解析响应JSON失败: %w", err)
	}

	if apiResponse.Status != "success" {
		return nil, fmt.Errorf("API返回错误: %s", apiResponse.Message)
	}

	holePunchInfo := &domain.HolePunchInfo{}
	if len(apiResponse.Data) == 0 || string(apiResponse.Data) == "null" {
		return holePunchInfo, nil
	}

	if err := json.Unmarshal(apiResponse.Data, holePunchInfo); err == nil {
		if !holePunchInfo.HasRequest {
			return holePunchInfo, nil
		}
		return holePunchInfo, nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal(apiResponse.Data, &data); err != nil {
		return nil, fmt.Errorf("响应数据格式错误: %w", err)
	}

	if hasRequest, ok := parseBool(data["has_request"]); ok {
		holePunchInfo.HasRequest = hasRequest
	}
	if !holePunchInfo.HasRequest {
		return holePunchInfo, nil
	}

	if requesterKey, ok := data["requester_key"].(string); ok {
		holePunchInfo.RequesterKey = requesterKey
	}
	if externalIP, ok := data["external_ip"].(string); ok {
		holePunchInfo.ExternalIP = externalIP
	}
	if externalPort, ok := parseString(data["external_port"]); ok {
		holePunchInfo.ExternalPort = externalPort
	}
	if localIP, ok := data["local_ip"].(string); ok {
		holePunchInfo.LocalIP = localIP
	}
	if localPort, ok := parseString(data["local_port"]); ok {
		holePunchInfo.LocalPort = localPort
	}
	if timestamp, ok := parseInt64(data["timestamp"]); ok {
		holePunchInfo.Timestamp = timestamp
	}

	return holePunchInfo, nil
}

func parseBool(value interface{}) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		if typed == "true" {
			return true, true
		}
		if typed == "false" {
			return false, true
		}
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err == nil {
			return parsed != 0, true
		}
	case float64:
		return typed != 0, true
	case int:
		return typed != 0, true
	case int64:
		return typed != 0, true
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parsed != 0, true
		}
	}
	return false, false
}

func parseString(value interface{}) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, true
	case float64:
		return strconv.FormatInt(int64(typed), 10), true
	case int:
		return strconv.Itoa(typed), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	case json.Number:
		return typed.String(), true
	}
	return "", false
}

func parseInt64(value interface{}) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), true
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err == nil {
			return parsed, true
		}
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}
