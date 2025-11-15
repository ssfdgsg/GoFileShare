# WebRTC P2P NAT穿透快速开始指南

## 简述

为GoFileShare添加了WebRTC支持，用于穿透NAT并建立P2P连接。现在支持两种P2P连接方式：
- **WebRTC**: 现代化方案，自动NAT穿透，支持数据通道和HTTP传输
- **QUIC**: 高性能方案，基于UDP的可靠传输

## 快速体验

### 1. 混合模式连接（推荐）

最简单的方式 - 系统自动选择最优连接方案：

```bash
# 初始化P2P服务
curl -X POST http://localhost:8080/api/p2p/register

# 连接到目标客户端（自动尝试WebRTC，失败时回退到QUIC）
curl -X POST http://localhost:8080/api/p2p/hybrid-connect \
  -H "Content-Type: application/json" \
  -d '{
    "target_key": "目标客户端的密钥",
    "preferred_mode": "auto"
  }'
```

### 2. 纯WebRTC连接

使用WebRTC建立更稳定的连接：

```bash
# A端（发起方）
curl -X POST http://localhost:8080/api/webrtc/init

curl -X POST http://localhost:8080/api/webrtc/offer
# 获取 offer JSON，发送给B端

# B端（响应方）
curl -X POST http://localhost:8080/api/webrtc/init

curl -X POST http://localhost:8080/api/webrtc/remote-offer \
  -H "Content-Type: application/json" \
  -d '{"offer": "A端的Offer JSON"}'
# 获取 answer JSON，发送回A端

# A端接收answer
curl -X POST http://localhost:8080/api/webrtc/answer \
  -H "Content-Type: application/json" \
  -d '{"answer": "B端的Answer JSON"}'

# 连接建立，可以传输数据
curl -X POST http://localhost:8080/api/webrtc/message \
  -H "Content-Type: application/json" \
  -d '{"label": "file-transfer", "message": "Hello"}'
```

### 3. 查询连接方式

```bash
# 获取可用的连接方式
curl http://localhost:8080/api/p2p/methods

# 测试所有连接方式的可用性
curl -X POST http://localhost:8080/api/p2p/test-methods \
  -H "Content-Type: application/json" \
  -d '{"target_key": "目标密钥"}'
```

## 文件结构

### 新增服务文件
- `services/webrtc.go` - WebRTC P2P服务核心实现
- `services/webrtc_signaling.go` - WebRTC信令管理

### 新增控制器
- `controllers/webrtc_controller.go` - WebRTC基础API
- `controllers/webrtc_signaling_controller.go` - 信令交换API
- `controllers/hybrid_p2p_controller.go` - 混合P2P管理

### 新增文档
- `WEBRTC_IMPLEMENTATION.md` - 完整实现指南
- `WEBRTC_QUICK_START.md` - 本文件

## 核心优势

### WebRTC相比QUIC
✅ 自动NAT穿透（ICE候选收集）
✅ 支持TURN中继（QUIC暂不支持）
✅ 标准化协议（RFC 8445）
✅ 更多传输选项

### 同时支持两种方式
✅ 可靠性提升 - 失败自动回退
✅ 兼容性提升 - 适应各种NAT环境
✅ 灵活性提升 - 用户可选择

## NAT穿透工作原理

### WebRTC工作流程

```
┌─────────────────────────────────────────────────┐
│ 1. STUN探测                                      │
│    - 检测外网IP和端口                            │
│    - 确定NAT类型                                │
└─────────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────────┐
│ 2. ICE候选收集                                   │
│    - 本地地址候选                                │
│    - 外网地址候选 (srflx)                        │
│    - 中继地址候选 (relay)                        │
└─────────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────────┐
│ 3. SDP交换 (通过信令)                           │
│    - A发送Offer                                │
│    - B收到Offer，生成Answer                     │
│    - A收到Answer                               │
└─────────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────────┐
│ 4. 候选对配对                                    │
│    - 尝试不同的local/remote候选组合             │
│    - 连接性检查                                 │
│    - 选择最优路径                               │
└─────────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────────┐
│ 5. 连接建立                                      │
│    - P2P直连 或 TURN中继                        │
│    - 数据通道就绪                               │
└─────────────────────────────────────────────────┘
```

## HTTP over WebRTC

支持在WebRTC数据通道上运行HTTP协议，实现：
- 文件传输（GET/POST）
- 实时通信
- 避免TCP端口开放

```javascript
// 发送HTTP请求示例
await fetch('/api/webrtc/message', {
  method: 'POST',
  body: JSON.stringify({
    label: 'file-transfer',
    message: JSON.stringify({
      method: 'GET',
      path: '/api/files',
      body: null
    })
  })
});
```

## 配置参数

### STUN服务器（自动配置）
- stun.l.google.com:19302
- stun1.l.google.com:19302
- stun2.l.google.com:19302
- stun3.l.google.com:19302
- stun4.l.google.com:19302

### ICE超时
- HandshakeIdleTimeout: 30秒
- MaxIdleTimeout: 60秒
- KeepAlivePeriod: 10秒

## 性能建议

1. **内存使用**
   - 数据通道缓冲区有限
   - 大文件分块处理

2. **CPU使用**
   - ICE检测可能占用CPU
   - 合理设置超时

3. **带宽使用**
   - STUN请求最小化
   - 候选收集完成后停止

## 故障排查

### WebRTC连接失败
```
1. 检查STUN服务器可达性
2. 检查防火墙UDP规则
3. 查看日志中的错误信息
4. 尝试增加超时时间
```

### 候选收集超时
```
1. 检查网络连接
2. 确认STUN服务器可用
3. 检查是否处于极限NAT环境
4. 配置TURN服务器作为回退
```

## 信令服务器API

用于offer/answer交换的API：

```
POST /api/signaling/register          - 注册客户端
GET  /api/signaling/client-info       - 获取客户端信息
POST /api/signaling/offer             - 发送Offer
POST /api/signaling/answer            - 发送Answer
POST /api/signaling/candidate         - 发送ICE候选
GET  /api/signaling/messages          - 获取消息
POST /api/signaling/unregister        - 注销客户端
```

## 浏览器集成

在Web前端使用WebRTC进行P2P连接（需要WASM编译）：

```javascript
// 初始化
initWASM();

// 建立WebRTC连接
await connectViaPeerConnection({
    targetPeerId: 'target_key',
    preferredMode: 'webrtc'
});

// 监听数据通道消息
onDataChannelMessage((label, data) => {
    console.log('收到数据:', label, data);
});

// 发送数据
sendViaPeerConnection('file-transfer', messageData);
```

## 部署注意事项

### 生产环境建议

1. **配置私有TURN服务器**
   - 处理对称NAT
   - 提高连接成功率

2. **使用HTTPS**
   - 信令传输安全
   - 符合浏览器WebRTC要求

3. **监控连接质量**
   - 记录连接失败
   - 分析NAT穿透成功率

4. **日志记录**
   - 调试ICE候选问题
   - 监控系统性能

## 更多信息

详见 `WEBRTC_IMPLEMENTATION.md` 获取完整的技术文档和API参考。

## 相关RFC标准

- RFC 8445 - Interactive Connectivity Establishment (ICE)
- RFC 5245 - Interactive Connectivity Establishment (ICE) - 旧版本
- RFC 5128 - State of Peer-to-Peer Communication across NATs
- W3C WebRTC规范

## 许可证

遵循原项目许可证
