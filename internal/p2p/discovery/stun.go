package discovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"time"

	"GoFileShare/internal/domain"

	"github.com/denisbrodbeck/machineid"
	"github.com/pion/stun"
)

type STUNDiscovery struct {
	servers   []string
	listenPort int
}

func NewSTUNDiscovery(servers []string, listenPort int) *STUNDiscovery {
	return &STUNDiscovery{servers: servers, listenPort: listenPort}
}

func (d *STUNDiscovery) Discover(ctx context.Context) (*domain.P2PInfo, error) {
	if len(d.servers) == 0 {
		return nil, fmt.Errorf("stun server list is empty")
	}

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: d.listenPort})
	if err != nil {
		return nil, fmt.Errorf("创建UDP连接失败: %w", err)
	}
	defer conn.Close()

	var lastErr error
	for _, stunServer := range d.servers {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		serverAddr, err := net.ResolveUDPAddr("udp", stunServer)
		if err != nil {
			lastErr = fmt.Errorf("解析STUN服务器%s地址失败: %w", stunServer, err)
			continue
		}

		message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
		_, err = conn.WriteTo(message.Raw, serverAddr)
		if err != nil {
			lastErr = fmt.Errorf("发送STUN请求到%s失败: %w", stunServer, err)
			continue
		}

		buf := make([]byte, 1500)
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			lastErr = fmt.Errorf("读取STUN响应失败: %w", err)
			continue
		}

		res := new(stun.Message)
		res.Raw = buf[:n]
		if err := res.Decode(); err != nil {
			lastErr = fmt.Errorf("解码STUN响应失败: %w", err)
			continue
		}

		var mappedAddr stun.XORMappedAddress
		if err := mappedAddr.GetFrom(res); err != nil {
			lastErr = fmt.Errorf("获取映射地址失败: %w", err)
			continue
		}

		machineID, err := machineid.ID()
		if err != nil {
			return nil, fmt.Errorf("无法获取本机的Machine ID: %w", err)
		}
		macs := ""
		ifaces, err := net.Interfaces()
		if err == nil {
			for _, iface := range ifaces {
				macs += iface.HardwareAddr.String()
			}
		}
		keySource := machineID + macs + mappedAddr.IP.String() + strconv.Itoa(mappedAddr.Port)
		hash := sha256.Sum256([]byte(keySource))
		uniqueKey := hex.EncodeToString(hash[:])

		return &domain.P2PInfo{
			OutIP:   mappedAddr.IP.String(),
			OutPort: strconv.Itoa(mappedAddr.Port),
			Key:     uniqueKey,
		}, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("all stun servers failed")
	}
	return nil, lastErr
}
