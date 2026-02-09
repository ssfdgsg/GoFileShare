package domain

import "context"

// P2PInfo captures local external-facing identity.
type P2PInfo struct {
	OutIP   string
	OutPort string
	Key     string
}

// Discovery resolves NAT/external address information.
type Discovery interface {
	Discover(ctx context.Context) (*P2PInfo, error)
}

// Signaling registers and queries hole-punch info.
type Signaling interface {
	Register(ctx context.Context, info P2PInfo) error
	GetHolePunch(ctx context.Context, targetKey string) (*HolePunchInfo, error)
}

// Transport handles UDP/QUIC connectivity and response listeners.
type Transport interface {
	StartResponseListener(port int, key string) error
	StopResponseListener()
	ConnectPeerTest(ctx context.Context, key string, info *HolePunchInfo) error
	ConnectPeer(ctx context.Context, key string, info *HolePunchInfo) error
}
