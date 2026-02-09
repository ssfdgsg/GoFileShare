package manager

import (
	"context"
	"strconv"
	"time"

	"GoFileShare/internal/domain"
)

type P2PManager struct {
	discovery    domain.Discovery
	signaling    domain.Signaling
	transport    domain.Transport
	info         *domain.P2PInfo
	registeredAt int64
}

func NewP2PManager(discovery domain.Discovery, signaling domain.Signaling, transport domain.Transport) *P2PManager {
	return &P2PManager{discovery: discovery, signaling: signaling, transport: transport}
}

func (m *P2PManager) Init(ctx context.Context) (*domain.P2PInfo, error) {
	info, err := m.discovery.Discover(ctx)
	if err != nil {
		return nil, err
	}
	if m.info != nil && m.info.Key != "" {
		info.Key = m.info.Key
	}
	m.info = info
	return info, nil
}

func (m *P2PManager) OverrideKey(key string) {
	if key == "" {
		return
	}
	if m.info == nil {
		m.info = &domain.P2PInfo{}
	}
	m.info.Key = key
}

func (m *P2PManager) Register(ctx context.Context) error {
	if m.info == nil {
		return domainError("P2PService未初始化")
	}
	if err := m.signaling.Register(ctx, *m.info); err != nil {
		return err
	}
	port, err := strconv.Atoi(m.info.OutPort)
	if err != nil {
		return err
	}
	if err := m.transport.StartResponseListener(port, m.info.Key); err != nil {
		return err
	}
	m.registeredAt = time.Now().Unix()
	return nil
}

func (m *P2PManager) GetHolePunch(ctx context.Context, targetKey string) (*domain.HolePunchInfo, error) {
	return m.signaling.GetHolePunch(ctx, targetKey)
}

func (m *P2PManager) ConnectPeerTest(ctx context.Context, info *domain.HolePunchInfo) error {
	if m.info == nil {
		return domainError("P2P服务未初始化")
	}
	return m.transport.ConnectPeerTest(ctx, m.info.Key, info)
}

func (m *P2PManager) ConnectPeer(ctx context.Context, info *domain.HolePunchInfo) error {
	if m.info == nil {
		return domainError("P2P服务未初始化")
	}
	return m.transport.ConnectPeer(ctx, m.info.Key, info)
}

func (m *P2PManager) Info() *domain.P2PInfo {
	return m.info
}

func (m *P2PManager) RegisteredAtUnix() int64 {
	return m.registeredAt
}

func domainError(message string) error {
	return &managerError{message: message}
}

type managerError struct {
	message string
}

func (e *managerError) Error() string {
	return e.message
}
