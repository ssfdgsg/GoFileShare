//go:build windows

package services

import (
	"fmt"
	"syscall"
)

// setSocketOptions 为Windows系统设置socket选项
// Windows上没有SO_REUSEPORT，因此仅设置SO_REUSEADDR
func (tc *TargetClient) setSocketOptions(network, address string, c syscall.RawConn) error {
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
