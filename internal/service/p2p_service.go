package service

import (
	"context"

	"GoFileShare/internal/domain"
	"GoFileShare/internal/p2p/manager"
)

type P2PService struct {
	manager *manager.P2PManager
}

func NewP2PService(manager *manager.P2PManager) *P2PService {
	return &P2PService{manager: manager}
}

func (s *P2PService) Init(ctx context.Context) (*domain.P2PInfo, error) {
	return s.manager.Init(ctx)
}

func (s *P2PService) Register(ctx context.Context) error {
	return s.manager.Register(ctx)
}

func (s *P2PService) OverrideKey(key string) {
	s.manager.OverrideKey(key)
}

func (s *P2PService) RegisteredAtUnix() int64 {
	return s.manager.RegisteredAtUnix()
}

func (s *P2PService) GetHolePunch(ctx context.Context, targetKey string) (*domain.HolePunchInfo, error) {
	return s.manager.GetHolePunch(ctx, targetKey)
}

func (s *P2PService) ConnectPeerTest(ctx context.Context, info *domain.HolePunchInfo) error {
	return s.manager.ConnectPeerTest(ctx, info)
}

func (s *P2PService) ConnectPeer(ctx context.Context, info *domain.HolePunchInfo) error {
	return s.manager.ConnectPeer(ctx, info)
}

func (s *P2PService) Info() *domain.P2PInfo {
	return s.manager.Info()
}
