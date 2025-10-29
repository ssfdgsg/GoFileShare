//go:build unix || linux || darwin

package services

import (
    "fmt"
    "syscall"

    "golang.org/x/sys/unix"
)

// setSocketOptions 为Unix/Linux/macOS系统设置socket选项以支持NAT穿透
func (tc *TargetClient) setSocketOptions(network, address string, c syscall.RawConn) error {
    var sockOptErr error
    err := c.Control(func(fd uintptr) {
        // 设置SO_REUSEADDR - 允许地址重用
        if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
            sockOptErr = fmt.Errorf("无法设置SO_REUSEADDR: %w", err)
            return
        }

        // 设置SO_REUSEPORT - 允许端口重用（Linux 3.9+, macOS等）
        // 这对于NAT穿透很重要，允许多个socket绑定同一端口
        if err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1); err != nil && err != unix.ENOPROTOOPT {
            // SO_REUSEPORT在某些系统上可能不可用，记录但不中断流程
            sockOptErr = fmt.Errorf("警告: 无法设置SO_REUSEPORT: %w", err)
        }
    })

    if err != nil {
        return fmt.Errorf("socket控制失败: %w", err)
    }

    return sockOptErr
}
