# WebRTC P2P NAT穿透实现指南

## 概述

本项目在现有QUIC P2P基础上添加了WebRTC支持，提供一种更强大的NAT穿透和P2P连接方案。两种方案可以独立使用，也可以混合使用自动选择最优方案。

## 核心特性

### 1. **WebRTC P2P连接**
- 基于Pion WebRTC库的完整实现
- 自动ICE候选收集和选择
- 支持STUN服务器进行NAT检测
- 自动选择最优连接路径（UDP直连或TURN中继）
- 内置数据通道支持

### 2. **HTTP over WebRTC**
- 在WebRTC数据通道中运行HTTP协议
- 支持文件传输和其他基于HTTP的操作
- 自动序列化/反序列化HTTP请求和响应
- 避免直接开放TCP端口的安全性问题

### 3. **混合P2P模式**
- 自动尝试WebRTC连接
- 失败时回退到QUIC
- 支持指定优先连接方式
- 连接状态监控

## API端点

### WebRTC基础API

#### 初始化WebRTC服务
```
POST /api/webrtc/init
响应: { status, message, connection }
```

#### 创建Offer
```
POST /api/webrtc/offer
响应: { status, offer }
```

#### 处理Answer
```
POST /api/webrtc/answer
请求体: { answer }
响应: { status, message }
```

#### 处理远程Offer
```
POST /api/webrtc/remote-offer
请求体: { offer }
响应: { status, message, answer }
```

#### 添加ICE候选
```
POST /api/webrtc/candidate
请求体: { candidate }
响应: { status, message }
```

#### 获取ICE候选列表
```
GET /api/webrtc/candidates
响应: { status, candidates[] }
```

#### 获取连接状态
```
GET /api/webrtc/status
响应: { status, data: { initialized, connection_state } }
```

#### 发送消息
```
POST /api/webrtc/message
请求体: { label, message }
响应: { status, message }
```

#### 关闭连接
```
POST /api/webrtc/close
响应: { status, message }
```

### WebRTC信令API

#### 注册信令客户端
```
POST /api/signaling/register
请求体: { client_id, external_ip, external_port }
响应: { status, message }
```

#### 获取客户端信息
```
GET /api/signaling/client-info?client_id=xxx
响应: { status, data: { client_id, external_ip, external_port, connected } }
```

#### 交换Offer
```
POST /api/signaling/offer
请求体: { from_client_id, to_client_id, sdp }
响应: { status, message }
```

#### 交换Answer
```
POST /api/signaling/answer
请求体: { from_client_id, to_client_id, sdp }
响应: { status, message }
```

#### 交换ICE候选
```
POST /api/signaling/candidate
请求体: { from_client_id, to_client_id, candidate, sdp_mline_index, sdp_mid }
响应: { status, message }
```

### 混合P2P API

#### 混合方式连接
```
POST /api/p2p/hybrid-connect
请求体: { target_key, preferred_mode }
  - preferred_mode: "webrtc", "quic", "auto" (默认: "auto")
响应: { status, message, preferred_mode, target_ip, target_port }
```

#### 获取可用连接方式
```
GET /api/p2p/methods
响应: { status, methods: { webrtc, quic, hybrid } }
```

#### 测试所有P2P方式
```
POST /api/p2p/test-methods
请求体: { target_key }
响应: { status, results: { webrtc, quic }, recommended }
```

#### 查询P2P连接信息
```
GET /api/p2p/info?target_key=xxx
响应: { status, info: HolePunchInfo }
```

## 使用流程

### 1. WebRTC直连流程

#### 发起端 (Alice)
```
1. POST /api/webrtc/init                    -> 初始化WebRTC服务
2. POST /api/webrtc/offer                   -> 创建Offer
3. [通过信令服务器发送Offer给Bob]
4. [等待Bob的Answer]
5. POST /api/webrtc/answer (body: answer)   -> 处理Answer
6. GET /api/webrtc/candidates               -> 获取本地ICE候选
7. [发送候选给Bob]
8. POST /api/webrtc/message                 -> 发送消息/HTTP请求
```

#### 响应端 (Bob)
```
1. POST /api/webrtc/init                    -> 初始化WebRTC服务
2. [接收Alice的Offer]
3. POST /api/webrtc/remote-offer            -> 处理Offer并生成Answer
4. [通过信令服务器发送Answer给Alice]
5. POST /api/webrtc/candidate               -> 接收和添加ICE候选
6. [等待数据通道建立]
7. 接收消息（通过OnMessage回调）
```

### 2. 混合P2P流程

```
1. 注册P2P客户端: POST /api/p2p/register
2. 连接对端: POST /api/p2p/hybrid-connect (preferred_mode: "auto")
   - 系统自动尝试WebRTC
   - 如果失败，自动回退到QUIC
3. 连接建立后进行数据传输
```

## 信令交换示例

### 完整的WebRTC连接建立流程

```javascript
// Alice (发起端) 的浏览器代码示例
async function initiateConnection(targetBobId) {
    // 1. 初始化
    await fetch('/api/webrtc/init', { method: 'POST' });
    
    // 2. 创建Offer
    const offerResp = await fetch('/api/webrtc/offer', { method: 'POST' });
    const { offer } = await offerResp.json();
    
    // 3. 注册客户端
    await fetch('/api/signaling/register', {
        method: 'POST',
        body: JSON.stringify({
            client_id: 'alice_' + Date.now(),
            external_ip: '公网IP',
            external_port: '公网端口'
        })
    });
    
    // 4. 发送Offer给Bob（通过信令）
    await fetch('/api/signaling/offer', {
        method: 'POST',
        body: JSON.stringify({
            from_client_id: 'alice_...',
            to_client_id: targetBobId,
            sdp: JSON.parse(offer).sdp
        })
    });
    
    // 5. 监听ICE候选
    const candidatesResp = await fetch('/api/webrtc/candidates');
    const { candidates } = await candidatesResp.json();
    
    // 6. 发送候选
    for (const candidate of candidates) {
        const parsed = JSON.parse(candidate);
        if (parsed.type === 'candidate') {
            await fetch('/api/signaling/candidate', {
                method: 'POST',
                body: JSON.stringify({
                    from_client_id: 'alice_...',
                    to_client_id: targetBobId,
                    ...parsed.candidate
                })
            });
        }
    }
    
    // 7. 等待Bob的Answer（从信令服务器获取）
    // [轮询或WebSocket监听消息]
    const answerMsg = await pollMessages();
    
    // 8. 处理Answer
    await fetch('/api/webrtc/answer', {
        method: 'POST',
        body: JSON.stringify({ answer: answerMsg.sdp })
    });
    
    // 9. 连接建立，可以进行数据传输
    await fetch('/api/webrtc/message', {
        method: 'POST',
        body: JSON.stringify({
            label: 'file-transfer',
            message: 'Hello from Alice!'
        })
    });
}

// Bob (响应端) 的处理
async function respondToConnection(aliceOffer) {
    // 1. 初始化
    await fetch('/api/webrtc/init', { method: 'POST' });
    
    // 2. 处理Offer并生成Answer
    const answerResp = await fetch('/api/webrtc/remote-offer', {
        method: 'POST',
        body: JSON.stringify({ offer: aliceOffer })
    });
    const { answer } = await answerResp.json();
    
    // 3. 发送Answer回给Alice
    await fetch('/api/signaling/answer', {
        method: 'POST',
        body: JSON.stringify({
            from_client_id: 'bob_...',
            to_client_id: 'alice_...',
            sdp: JSON.parse(answer).sdp
        })
    });
    
    // 4. 发送候选
    // [类似Alice的流程]
    
    // 5. 等待连接建立
    // [连接建立后自动接收数据]
}
```

## NAT穿透机制

### WebRTC的NAT穿透优势

1. **自动ICE候选**
   - 收集所有可用的地址候选（host、srflx、prflx、relay）
   - 自动尝试多种连接路径
   - 优先级策略确保最优路由

2. **STUN/TURN支持**
   - 使用Google STUN服务器进行NAT映射检测
   - 如果直连失败，自动使用TURN中继

3. **并发连接尝试**
   - 发起端和响应端同时尝试建立连接
   - 增加穿透成功率

4. **候选配对**
   - 系统自动尝试不同的local/remote候选配对
   - 找到最优的传输路径

### QUIC的NAT穿透机制（现有）

1. **UDP打洞**
   - 双方同时向对方的公网地址发送数据包

2. **Socket重用**
   - SO_REUSEADDR和SO_REUSEPORT支持

3. **持续打洞**
   - 连接建立前持续发送打洞包

## 配置说明

### STUN服务器列表
```go
[]string{
    "stun:stun.l.google.com:19302",
    "stun:stun1.l.google.com:19302",
    "stun:stun2.l.google.com:19302",
    "stun:stun3.l.google.com:19302",
    "stun:stun4.l.google.com:19302",
}
```

### ICE检测超时
- HandshakeIdleTimeout: 30秒
- MaxIdleTimeout: 60秒

## 测试建议

### 1. 相同NAT场景测试
```bash
# 两个客户端在同一局域网
curl -X POST http://localhost:8080/api/p2p/hybrid-connect \
  -H "Content-Type: application/json" \
  -d '{"target_key":"xxx","preferred_mode":"auto"}'
```

### 2. 不同NAT场景测试
- 使用实际的公网IP进行测试

### 3. 连接方式对比
```bash
# 测试所有P2P方式
curl -X POST http://localhost:8080/api/p2p/test-methods \
  -H "Content-Type: application/json" \
  -d '{"target_key":"xxx"}'
```

## 性能考虑

1. **CPU使用**
   - WebRTC ICE检测会占用一定CPU
   - 合理设置超时时间

2. **带宽使用**
   - STUN请求最小化
   - 候选收集完成后停止探测

3. **内存使用**
   - 数据通道缓冲区大小有限
   - 大文件传输应分块处理

## 故障排查

### WebRTC连接失败

1. **STUN服务器不可达**
   - 检查网络连接
   - 尝试其他STUN服务器

2. **NAT类型限制**
   - 对称NAT可能无法直连
   - 应配置TURN服务器作为回退

3. **防火墙阻止**
   - 检查防火墙规则
   - 允许UDP通讯

### 候选收集超时

- 增加GatheringCompletePromise的等待时间
- 检查网络状态

## 未来改进

1. **TURN服务器支持**
   - 配置私有TURN服务器
   - 提高对称NAT穿透率

2. **WebSocket信令**
   - 替代HTTP轮询
   - 实时消息推送

3. **数据通道优化**
   - 实现流式传输
   - 优化带宽使用

4. **性能监控**
   - 连接质量评估
   - 丢包率统计

## 参考文档

- [RFC 8445 - ICE](https://tools.ietf.org/html/rfc8445)
- [WebRTC规范](https://www.w3.org/TR/webrtc/)
- [Pion WebRTC文档](https://github.com/pion/webrtc)
- [RFC 5128 - NAT穿透](https://tools.ietf.org/html/rfc5128)
