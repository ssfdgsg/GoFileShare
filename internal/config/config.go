package config

import (
	"errors"
	"GoFileShare/utils"
)

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

type MongoConfig struct {
	URL      string
	User     string
	Password string
	Host     string
	Port     string
}

type P2PConfig struct {
	ServerIP   string
	ServerPort string
	ListenPort int
	QUICPort   int
	STUNServers []string
}

type TransferConfig struct {
	MetaDir     string
	WorkerCount int
	ChunkSize   int64
}

type AppConfig struct {
	Database DatabaseConfig
	Mongo    MongoConfig
	P2P      P2PConfig
	Transfer TransferConfig
	RootPath string
}

func Load() (*AppConfig, error) {
	listenPort := 8080
	cfg := &AppConfig{
		Database: DatabaseConfig{
			Host:     utils.GetEnv("DB_HOST", "localhost"),
			Port:     utils.GetEnv("DB_PORT", "3306"),
			User:     utils.GetEnv("DB_USER", "root"),
			Password: utils.GetEnv("DB_PASSWORD", "123456"),
			Name:     utils.GetEnv("DB_NAME", "gotest"),
		},
		Mongo: MongoConfig{
			URL:      utils.GetEnv("MONGO_URL", ""),
			User:     utils.GetEnv("MONGO_USER", ""),
			Password: utils.GetEnv("MONGO_PASSWORD", ""),
			Host:     utils.GetEnv("MONGO_HOST", ""),
			Port:     utils.GetEnv("MONGO_PORT", ""),
		},
		P2P: P2PConfig{
			ServerIP:   utils.GetEnv("P2P_SERVER_IP", "127.0.0.1"),
			ServerPort: utils.GetEnv("P2P_SERVER_PORT", "8888"),
			ListenPort: listenPort,
			QUICPort:   listenPort,
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
		},
		Transfer: TransferConfig{
			MetaDir:     "meta",
			WorkerCount: 4,
			ChunkSize:   1024 * 1024,
		},
		RootPath: ".",
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *AppConfig) Validate() error {
	if c.Database.Host == "" || c.Database.Port == "" || c.Database.User == "" || c.Database.Name == "" {
		return errors.New("database config is incomplete")
	}
	if c.Mongo.URL == "" {
		if c.Mongo.User == "" || c.Mongo.Password == "" || c.Mongo.Host == "" || c.Mongo.Port == "" {
			return errors.New("mongo config is incomplete")
		}
	}
	if c.P2P.ServerIP == "" || c.P2P.ServerPort == "" {
		return errors.New("p2p config is incomplete")
	}
	if c.P2P.ListenPort <= 0 || c.P2P.QUICPort <= 0 {
		return errors.New("p2p ports must be positive")
	}
	if len(c.P2P.STUNServers) == 0 {
		return errors.New("p2p stun server list is empty")
	}
	return nil
}
