//go:build windows

package transport

import (
	"context"
	"fmt"
	"net"
	"syscall"
)

// createReuseUDPConn creates a UDP connection with address reuse enabled.
func createReuseUDPConn(laddr *net.UDPAddr) (*net.UDPConn, error) {
	lc := net.ListenConfig{Control: setSocketOptions}
	conn, err := lc.ListenPacket(context.Background(), "udp", laddr.String())
	if err != nil {
		return nil, fmt.Errorf("创建UDP连接失败: %w", err)
	}

	udpConn, ok := conn.(*net.UDPConn)
	if !ok {
		conn.Close()
		return nil, fmt.Errorf("类型转换失败：无法转换为*net.UDPConn")
	}
	return udpConn, nil
}

func setSocketOptions(network, address string, c syscall.RawConn) error {
	var sockOptErr error
	err := c.Control(func(fd uintptr) {
		if err := syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
			sockOptErr = fmt.Errorf("无法设置SO_REUSEADDR: %w", err)
		}
	})
	if err != nil {
		return fmt.Errorf("socket控制失败: %w", err)
	}
	return sockOptErr
}
