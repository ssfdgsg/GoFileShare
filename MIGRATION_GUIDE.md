# DDD 到 MVC 架构迁移指南

## 概述

本项目已从 DDD（领域驱动设计）架构成功重构为 MVC（Model-View-Controller）架构。

## 主要变更

### 1. 目录结构变更

#### 已删除的目录
```
❌ internal/domain/          # 领域层
❌ internal/service/         # 服务层
❌ internal/repository/      # 仓储层
❌ internal/api/handlers/    # API处理器
❌ routes/                   # 旧路由
❌ cmd/server/               # 服务器入口（已合并到main.go）
```

#### 新增的目录
```
✅ controllers/              # 控制器层
   ├── auth_controller.go
   ├── file_controller.go
   └── p2p_controller.go
✅ models/                   # 模型层（重构）
   ├── user_model.go
   ├── file_model.go
   └── p2p_model.go
✅ router.go                 # 统一路由配置
```

### 2. 代码映射关系

#### 用户相关

**DDD 架构：**
```
internal/domain/repository.go (UserRepository接口)
    ↓
internal/repository/mysql/user_repository.go (实现)
    ↓
internal/service/user_service.go (业务逻辑)
    ↓
internal/api/handlers/auth.go (HTTP处理)
```

**MVC 架构：**
```
models/user_model.go (数据模型 + 数据库操作)
    ↓
controllers/auth_controller.go (HTTP处理 + 业务协调)
```

#### 文件相关

**DDD 架构：**
```
internal/domain/file.go (FileNode实体)
    ↓
internal/repository/mongo/file_repository.go (MongoDB操作)
    ↓
internal/service/file_service.go (文件业务逻辑)
    ↓
internal/api/handlers/user.go (文件HTTP处理)
```

**MVC 架构：**
```
models/file_model.go (FileNode模型 + MongoDB操作)
    ↓
controllers/file_controller.go (文件HTTP处理)
```

#### P2P相关

**DDD 架构：**
```
internal/domain/p2p.go (P2P接口定义)
    ↓
internal/p2p/discovery/stun.go (STUN发现)
internal/p2p/signaling/http.go (信令)
internal/p2p/transport/udp.go (传输)
    ↓
internal/p2p/manager/manager.go (P2P管理器)
    ↓
internal/service/p2p_service.go (P2P服务)
    ↓
internal/api/handlers/p2p.go (P2P HTTP处理)
```

**MVC 架构：**
```
models/p2p_model.go (P2P模型 + 所有P2P逻辑)
    ↓
controllers/p2p_controller.go (P2P HTTP处理)
```

### 3. 函数映射表

#### 用户操作

| DDD 函数 | MVC 函数 | 位置 |
|---------|---------|------|
| `userRepo.UserExists()` | `models.UserExists()` | models/user_model.go |
| `userRepo.CreateUser()` | `models.CreateUser()` | models/user_model.go |
| `userRepo.ValidateUser()` | `models.ValidateUser()` | models/user_model.go |
| `userRepo.GetUserByName()` | `models.GetUserByName()` | models/user_model.go |
| `userService.UserExists()` | `models.UserExists()` | models/user_model.go |
| `authHandler.Login()` | `authCtrl.Login()` | controllers/auth_controller.go |

#### 文件操作

| DDD 函数 | MVC 函数 | 位置 |
|---------|---------|------|
| `fileRepo.AddFileNode()` | `models.AddFileNode()` | models/file_model.go |
| `fileRepo.DeleteFileNode()` | `models.DeleteFileNode()` | models/file_model.go |
| `fileRepo.SearchFileNodeByID()` | `models.SearchFileNodeByID()` | models/file_model.go |
| `fileService.AddFileNode()` | `models.AddFileNode()` | models/file_model.go |
| `userHandler.StartUpload()` | `fileCtrl.StartUpload()` | controllers/file_controller.go |

#### P2P操作

| DDD 函数 | MVC 函数 | 位置 |
|---------|---------|------|
| `p2pManager.Init()` | `p2pManager.DiscoverP2PInfo()` | models/p2p_model.go |
| `p2pManager.Register()` | `p2pManager.StartResponseListener()` | models/p2p_model.go |
| `p2pService.ConnectPeer()` | `p2pManager.ConnectPeer()` | models/p2p_model.go |
| `p2pHandler.RegisterP2PKey()` | `p2pCtrl.RegisterP2PKey()` | controllers/p2p_controller.go |

### 4. 数据库变更

#### MySQL 表结构变更

**旧表名：** `user`  
**新表名：** `users`

**字段变更：**
| 旧字段名 | 新字段名 | 类型 |
|---------|---------|------|
| `name` | `username` | VARCHAR(100) |
| `create_time` | `created_at` | TIMESTAMP |
| `last_login` | `updated_at` | TIMESTAMP |
| `status` | `status` | INT (类型从TINYINT改为INT) |

**迁移SQL：**
```sql
-- 如果需要迁移旧数据
RENAME TABLE user TO users;
ALTER TABLE users CHANGE name username VARCHAR(100) NOT NULL;
ALTER TABLE users CHANGE create_time created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE users CHANGE last_login updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;
ALTER TABLE users MODIFY status INT DEFAULT 0;
```

#### MongoDB 集合

**集合名：** `FileDir` (保持不变)

**文档结构变更：**
```javascript
// 旧结构
{
  "_id": ObjectId,
  "name": String,
  "parent_id": String,
  "is_dir": Boolean,
  "effective_auth_level": Int,  // ❌ 已删除
  "auth_level": Int,             // ✅ 保留
  "storage": {
    "system_file_path": String,
    "size": Int64
  },
  "created_at": Date,
  "updated_at": Date
}

// 新结构（简化）
{
  "_id": ObjectId,
  "name": String,
  "parent_id": String,
  "is_dir": Boolean,
  "auth_level": Int,
  "storage": {
    "system_file_path": String,
    "size": Int64
  },
  "created_at": Date,
  "updated_at": Date
}
```

### 5. 配置变更

#### 环境变量 (.env)

保持不变，但建议添加：
```env
ROOT_PATH=.
```

#### 应用配置

**旧方式：**
```go
// internal/config/config.go
cfg, err := config.Load()
```

**新方式：**
```go
// config/app_config.go
appConfig := config.LoadAppConfig()
```

### 6. 路由变更

#### 旧路由配置
```go
// routes/routes.go
func SetupRouter(userService, fileService, p2pService, rootPath)
```

#### 新路由配置
```go
// router.go
func SetupRouter(rootPath, p2pServerIP, p2pServerPort)
```

### 7. 依赖注入变更

#### DDD 方式（复杂）
```go
// 创建仓储
userRepo := mysqlrepo.NewUserRepository(config.DB)
fileRepo := mongorepo.NewFileRepository(config.FileCollection, rootPath)

// 创建服务
userService := service.NewUserService(userRepo)
fileService := service.NewFileService(fileRepo)

// 创建处理器
authHandler := handlers.NewAuthHandler(userService)
userHandler := handlers.NewUserHandler(userService, fileService, rootPath)
```

#### MVC 方式（简化）
```go
// 设置全局数据库连接
models.SetDB(config.DB)
models.SetFileCollection(config.FileCollection, rootPath)
models.InitP2PManager(listenPort, quicPort, stunServers)

// 创建控制器
authCtrl := controllers.NewAuthController()
fileCtrl := controllers.NewFileController(rootPath)
p2pCtrl := controllers.NewP2PController(serverIP, serverPort)
```

### 8. 代码示例对比

#### 示例1：用户登录

**DDD 方式：**
```go
// internal/api/handlers/auth.go
func (h *AuthHandler) Login(c *gin.Context) {
    username := c.PostForm("user")
    password := c.PostForm("password")
    
    // 调用服务层
    valid, err := h.userService.ValidateUser(ctx, username, password)
    if err != nil {
        // 错误处理
    }
    
    // 调用服务层获取用户
    user, err := h.userService.GetUserByName(ctx, username)
    // ...
}
```

**MVC 方式：**
```go
// controllers/auth_controller.go
func (ctrl *AuthController) Login(c *gin.Context) {
    username := c.PostForm("user")
    password := c.PostForm("password")
    
    // 直接调用模型层
    valid, err := models.ValidateUser(ctx, username, password)
    if err != nil {
        // 错误处理
    }
    
    // 直接调用模型层获取用户
    user, err := models.GetUserByName(ctx, username)
    // ...
}
```

#### 示例2：文件上传

**DDD 方式：**
```go
// internal/api/handlers/user.go
func (h *UserHandler) StartUpload(c *gin.Context) {
    // ... 文件处理 ...
    
    // 调用服务层
    err := h.fileService.AddFileNode(ctx, filePath, fileName, false, parentID, auth)
}
```

**MVC 方式：**
```go
// controllers/file_controller.go
func (ctrl *FileController) StartUpload(c *gin.Context) {
    // ... 文件处理 ...
    
    // 直接调用模型层
    err := models.AddFileNode(ctx, filePath, fileName, false, parentID, auth)
}
```

### 9. 测试变更

#### 单元测试

**DDD 方式：** 需要 mock 多个层
```go
// 需要 mock Repository
mockRepo := &MockUserRepository{}
// 需要 mock Service
mockService := service.NewUserService(mockRepo)
// 测试 Handler
handler := handlers.NewAuthHandler(mockService)
```

**MVC 方式：** 只需 mock Model
```go
// 直接测试 Controller
ctrl := controllers.NewAuthController()
// 或者 mock models 包的函数
```

### 10. 性能影响

#### 优势
- ✅ 减少函数调用层次（3-4层 → 2层）
- ✅ 减少内存分配（少了中间对象）
- ✅ 代码更直观，编译器优化更好

#### 注意事项
- ⚠️ Model 层职责增加，需要注意代码组织
- ⚠️ 缺少接口抽象，测试时需要其他策略

### 11. 迁移检查清单

- [x] 删除 `internal/domain/` 目录
- [x] 删除 `internal/service/` 目录
- [x] 删除 `internal/repository/` 目录
- [x] 删除 `internal/api/handlers/` 目录
- [x] 删除 `routes/` 目录
- [x] 创建 `controllers/` 目录
- [x] 重构 `models/` 目录
- [x] 创建 `router.go`
- [x] 更新 `main.go`
- [x] 更新数据库表结构
- [x] 更新配置文件
- [x] 测试编译通过
- [ ] 运行集成测试
- [ ] 更新文档

### 12. 回滚方案

如果需要回滚到 DDD 架构：

```bash
# 使用 Git 回滚
git checkout <commit-before-refactor>

# 或者从备份恢复
# 确保在重构前创建了分支或标签
git checkout -b ddd-backup
```

### 13. 后续优化建议

1. **添加接口层**：为 Model 添加接口，便于测试
2. **拆分大文件**：如果 Model 文件过大，可以按功能拆分
3. **添加缓存层**：在 Model 和数据库之间添加缓存
4. **优化错误处理**：统一错误码和错误消息
5. **添加日志**：在关键操作点添加日志记录

### 14. 常见问题

**Q: 为什么要从 DDD 重构为 MVC？**  
A: DDD 适合复杂业务领域，但本项目业务逻辑相对简单，MVC 更直观易维护。

**Q: 重构后性能有提升吗？**  
A: 理论上有小幅提升（减少了函数调用层次），但主要优势在于代码简化。

**Q: 如何处理复杂业务逻辑？**  
A: 可以在 Model 中创建辅助函数，或者在 Controller 中组合多个 Model 操作。

**Q: 测试怎么办？**  
A: 可以使用 sqlmock 和 mongomock 来测试 Model 层，或者使用集成测试。

### 15. 联系方式

如有问题，请查看：
- 架构文档：`MVC_ARCHITECTURE.md`
- 代码注释：各文件中的详细注释
- Git 提交历史：查看重构过程

---

**重构完成日期：** 2026-01-25  
**重构人员：** Kiro AI Assistant  
**版本：** v2.0.0 (MVC)
