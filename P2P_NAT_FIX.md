# QUIC P2P NAT穿透超时问题修复

## 问题描述

当两个客户端（Nat1和Nat2）交换了彼此的NAT转换后的公网IP和端口，并尝试建立QUIC P2P连接时，会出现超时问题，无法正常连接。

## 根本原因

1. **Socket地址重用问题**: UDP socket没有设置`SO_REUSEADDR`和`SO_REUSEPORT`选项，这对NAT穿透至关重要
2. **NAT打洞时机不当**: 打洞包发送完成后立即尝试建立QUIC连接，没有持续打洞
3. **Simultaneous Open不完善**: 客户端拨号延迟过长（1秒），不利于对称NAT穿透
4. **超时时间过短**: 原有的10秒握手超时和30秒总超时对NAT穿透场景不够

## 解决方案

### 1. Socket选项配置（关键）

为UDP socket设置地址和端口重用选项：
- **SO_REUSEADDR**: 允许地址重用
- **SO_REUSEPORT**: 允许端口重用（Linux 3.9+, macOS等）

这些选项使得：
- 同一端口可以被多次绑定
- NAT设备能够正确映射和转发数据包
- 支持simultaneous open连接模式

实现文件：
- `services/p2p_socket_unix.go` - Unix/Linux/macOS系统
- `services/p2p_socket_windows.go` - Windows系统

### 2. 持续NAT打洞机制

实现了`continuousPunching`函数，在QUIC连接建立期间持续发送打洞包：
- 每300毫秒发送一次打洞包
- 直到QUIC连接成功建立或超时
- 保持NAT映射活跃状态

### 3. 优化连接时序

- **减少拨号延迟**: 从1秒减少到200毫秒，实现更好的simultaneous open
- **增加握手超时**: HandshakeIdleTimeout从10秒增加到30秒
- **增加总超时**: 总连接超时从30秒增加到60秒
- **增加keepalive**: KeepAlivePeriod从5秒增加到10秒

### 4. 改进的连接建立流程

1. 创建配置了socket选项的UDP连接
2. 启动持续NAT打洞协程
3. 等待500ms让打洞包先发送
4. 同时启动QUIC监听器和拨号器
5. 拨号器仅延迟200ms启动，实现simultaneous open

## 代码变更

### 主要修改 - services/p2p.go

```go
// 1. 使用新的createReuseUDPConn创建UDP连接
udpConn, err := tc.createReuseUDPConn(localAddr)

// 2. 启动持续打洞
punchCtx, punchCancel := context.WithCancel(ctx)
defer punchCancel()
go tc.continuousPunching(punchCtx, udpConn, remoteAddr, punchMessage)

// 3. 缩短拨号延迟到200ms
time.Sleep(200 * time.Millisecond)

// 4. 增加超时配置
HandshakeIdleTimeout: 30 * time.Second
MaxIdleTimeout:       60 * time.Second
```

### 新增文件

1. **services/p2p_socket_unix.go** - Unix系统socket配置
2. **services/p2p_socket_windows.go** - Windows系统socket配置

## NAT穿透原理

### Simultaneous Open技术

两个位于NAT后的客户端需要：
1. 都向对方的公网地址:端口发送数据包（打洞）
2. 几乎同时尝试建立连接
3. 使用相同的本地端口收发数据

这样NAT设备会：
1. 创建临时映射规则
2. 允许来自对方的数据包通过
3. 维持连接状态

### Socket重用的重要性

- **SO_REUSEADDR**: 允许bind到已使用的地址（TIME_WAIT状态）
- **SO_REUSEPORT**: 允许多个socket绑定同一端口（支持负载均衡和NAT穿透）

没有这些选项，不同的socket操作可能使用不同的本地端口，导致NAT映射失效。

## 测试建议

1. **相同NAT场景**: 两个客户端在同一NAT后（最简单）
2. **不同NAT场景**: 两个客户端在不同NAT后（标准场景）
3. **对称NAT场景**: 至少一方使用对称NAT（最困难）

监控日志关键信息：
- "持续发送打洞包" - 确认打洞正在进行
- "成功接受入站QUIC连接" 或 "成功建立出站QUIC连接" - 连接成功
- "连接超时" - 需要进一步调试

## 进一步优化建议

如果仍然遇到连接问题，可以考虑：

1. **TURN服务器回退**: 当直接P2P失败时，通过TURN中继
2. **ICE协议**: 实现完整的ICE协商流程
3. **更多STUN服务器**: 尝试多个STUN服务器以获取最佳映射
4. **端口预测**: 对于对称NAT，尝试预测下一个端口
5. **双向打洞**: 确保双方都在持续发送打洞包

## 参考资料

- [RFC 5128 - State of Peer-to-Peer Communication across NATs](https://tools.ietf.org/html/rfc5128)
- [RFC 8445 - Interactive Connectivity Establishment (ICE)](https://tools.ietf.org/html/rfc8445)
- [QUIC-GO Documentation](https://github.com/quic-go/quic-go)
