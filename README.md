# GoFileShare

一个基于Go语言开发的分布式文件共享系统，支持P2P文件传输、用户认证、文件管理等功能。

## 功能特性

- 🔐 **用户认证系统** - 支持用户注册、登录、会话管理
- 📁 **文件管理** - 文件上传、下载、分块传输
- 🌐 **P2P网络** - 点对点文件传输，支持NAT穿越
- 💾 **混合存储** - MySQL + MongoDB 双数据库架构
- 🐳 **容器化部署** - 完整的Docker Compose配置
- 🔄 **gRPC通信** - 高效的服务间通信
- 📊 **文件系统管理** - 层级文件结构管理

## 技术栈

- **后端框架**: Gin (Go)
- **数据库**: MySQL 8.0 + MongoDB 6.0
- **通信协议**: gRPC + HTTP/HTTPS
- **容器化**: Docker + Docker Compose
- **前端**: HTML模板 + Gin渲染
- **日志**: go-logger
- **会话管理**: gin-contrib/sessions

## 项目结构

```
GoFileShare/
├── main.go                 # 应用程序入口
├── go.mod                  # Go模块依赖
├── docker-compose.yml      # Docker编排配置
├── Dockerfile             # Docker构建文件
├── config/                # 配置管理
│   ├── database.go        # 数据库配置
│   └── storage.go         # 存储配置
├── controllers/           # 控制器层
│   ├── auth_controller.go # 认证控制器
│   ├── p2p_controller.go  # P2P控制器
│   └── user_controller.go # 用户控制器
├── models/                # 数据模型
│   ├── file.go           # 文件模型
│   ├── transfer.go       # 传输模型
│   └── user.go           # 用户模型
├── services/              # 业务逻辑
│   ├── file_service.go   # 文件服务
│   └── p2p.go           # P2P服务
├── handler/               # 处理器
│   └── filesHandler.go   # 文件处理器
├── middleware/            # 中间件
│   └── auth.go           # 认证中间件
├── routes/                # 路由配置
│   └── routes.go
├── proto/                 # gRPC协议文件
│   ├── callUpload.proto
│   ├── callUpload.pb.go
│   └── callUpload_grpc.pb.go
├── utils/                 # 工具库
│   ├── async.go          # 异步处理
│   ├── concurrent.go     # 并发控制
│   ├── dataStructure.go  # 数据结构
│   └── file_io.go        # 文件IO
├── views/                 # 前端模板
│   ├── home.html
│   ├── login.html
│   ├── register.html
│   ├── user.html
│   └── p2p_debug.html
└── FileStore/             # 文件存储目录
```

## 快速开始

### 前置要求

- Go 1.23.0+
- Docker & Docker Compose
- Git

### 环境配置

1. 克隆项目
```bash
git clone <repository-url>
cd GoFileShare
```

2. 创建环境配置文件 `.env`
```env
# 数据库配置
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=123456
DB_NAME=gotest

# MongoDB配置
MONGO_HOST=localhost
MONGO_PORT=27017
MONGO_DATABASE=filestore

# P2P服务器配置
P2P_SERVER_IP=127.0.0.1
P2P_SERVER_PORT=8888
```

### 使用Docker Compose部署（推荐）

```bash
# 启动所有服务
docker-compose up -d

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs app
```

服务将在以下端口启动：
- 应用服务: http://localhost:8080
- MySQL: localhost:3307
- MongoDB: localhost:27017

### 本地开发部署

1. 安装依赖
```bash
go mod download
```

2. 启动数据库服务
```bash
# 仅启动数据库
docker-compose up -d db filedb
```

3. 运行应用
```bash
go run main.go
```

## API接口

### 认证接口
- `GET /login` - 登录页面
- `POST /login` - 用户登录
- `GET /register` - 注册页面
- `POST /register` - 用户注册
- `POST /logout` - 用户登出

### 文件管理接口
- `POST /api/upload/init` - 初始化文件上传
- `POST /api/upload/chunk` - 分块上传
- `POST /api/upload/complete` - 完成上传
- `GET /api/upload/:id/status` - 获取上传状态
- `POST /api/download` - 初始化下载
- `GET /api/download/:id/file` - 下载文件
- `GET /api/download/:id/status` - 获取下载状态

### P2P接口
- `POST /p2p/connect` - P2P连接
- `GET /p2p/status` - P2P状态查询

## 配置说明

### 数据库配置
系统使用双数据库架构：
- **MySQL**: 存储用户信息、文件元数据
- **MongoDB**: 存储文件系统结构、传输记录

### P2P配置
支持NAT穿越的P2P文件传输，需要配置P2P服务器地址。

### 存储配置
- 默认文件存储目录: `./FileStore`
- 支持自定义存储路径
- 自动创建根目录结构

## 开发指南

### 添加新功能
1. 在 `models/` 中定义数据模型
2. 在 `services/` 中实现业务逻辑
3. 在 `controllers/` 中添加控制器
4. 在 `routes/` 中注册路由

### gRPC服务开发
```bash
# 生成gRPC代码
protoc --go_out=. --go_grpc_out=. proto/callUpload.proto
```

### 测试
```bash
# 运行测试
go test ./...

# 测试特定包
go test ./services
```

## 部署指南

### 生产环境部署

1. 修改 `docker-compose.yml` 中的数据库密码
2. 配置反向代理（推荐使用Nginx）
3. 设置SSL证书
4. 配置防火墙规则

### 性能优化
- 调整数据库连接池大小
- 配置文件上传大小限制
- 启用gzip压缩
- 配置缓存策略

## 故障排除

### 常见问题

1. **数据库连接失败**
   - 检查数据库服务是否启动
   - 验证连接配置
   - 查看防火墙设置

2. **文件上传失败**
   - 检查存储目录权限
   - 验证磁盘空间
   - 查看文件大小限制

3. **P2P连接失败**
   - 检查P2P服务器状态
   - 验证网络连通性
   - 查看NAT类型

### 日志查看
```bash
# Docker环境
docker-compose logs app

# 本地环境
# 日志输出到控制台
```

## 贡献指南

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 打开 Pull Request

## 许可证

本项目采用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。

## 联系方式

如有问题或建议，请提交 Issue 或 Pull Request。

---

**注意**: 这是一个开发中的项目，部分功能可能还在完善中。生产环境使用前请进行充分测试。
