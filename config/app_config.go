package config

import (
	"GoFileShare/utils"
)

// AppConfig 应用配置
type AppConfig struct {
	RootPath      string
	P2PServerIP   string
	P2PServerPort string
	ListenPort    int
	QUICPort      int
	STUNServers   []string
}

// LoadAppConfig 加载应用配置
func LoadAppConfig() *AppConfig {
	return &AppConfig{
		RootPath:      utils.GetEnv("ROOT_PATH", "."),
		P2PServerIP:   utils.GetEnv("P2P_SERVER_IP", "127.0.0.1"),
		P2PServerPort: utils.GetEnv("P2P_SERVER_PORT", "8888"),
		ListenPort:    8080,
		QUICPort:      8080,
		STUNServers: []string{
			"stun.l.google.com:19302",
			"stun1.l.google.com:19302",
			"stun2.l.google.com:19302",
			"stun3.l.google.com:19302",
			"stun4.l.google.com:19302",
			"stun.chat.bilibili.com:3478",
			"turn.cloudflare.com:3478",
			"stun.miwifi.com:3478",
		},
	}
}
