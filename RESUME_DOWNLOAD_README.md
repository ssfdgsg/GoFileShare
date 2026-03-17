# 断点续传下载功能

这个项目实现了完整的断点续传下载功能，支持将下载状态保存到本地文件，在程序重启后能够自动恢复未完成的下载。

## 功能特性

- ✅ **断点续传**: 支持在下载中断后从断点处继续下载
- ✅ **状态持久化**: 下载状态保存到本地JSON文件
- ✅ **并发下载**: 支持多线程分块下载，提高下载速度
- ✅ **自动恢复**: 程序启动时自动恢复未完成的下载
- ✅ **进度监控**: 实时显示下载进度
- ✅ **错误重试**: 下载失败时自动重试
- ✅ **文件验证**: 支持MD5哈希验证文件完整性
- ✅ **命令行工具**: 提供友好的CLI界面

## 核心组件

### 1. DownloadState (下载状态)
```go
type DownloadState struct {
    FileID       string    `json:"file_id"`       // 文件唯一标识
    FileName     string    `json:"file_name"`     // 文件名
    FilePath     string    `json:"file_path"`     // 本地文件路径
    FileSize     int64     `json:"file_size"`     // 文件总大小
    DownloadURL  string    `json:"download_url"`  // 下载URL
    Downloaded   int64     `json:"downloaded"`    // 已下载字节数
    Status       string    `json:"status"`        // 下载状态
    CreatedAt    time.Time `json:"created_at"`    // 创建时间
    UpdatedAt    time.Time `json:"updated_at"`    // 更新时间
    MD5Hash      string    `json:"md5_hash"`      // 文件MD5哈希
    ChunkSize    int64     `json:"chunk_size"`    // 分块大小
    Chunks       []Chunk   `json:"chunks"`        // 分块信息
}
```

### 2. DownloadStateManager (状态管理器)
负责管理下载状态的持久化存储：
- 创建和更新下载状态
- 保存状态到JSON文件
- 从文件加载状态
- 管理未完成的下载

### 3. ResumableDownloader (断点续传下载器)
核心下载引擎：
- 支持HTTP Range请求进行分块下载
- 并发下载多个分块
- 自动重试失败的分块
- 文件完整性验证

## 使用方法

### 1. 编程接口

```go
// 开始新的下载
err := utils.DownloadWithResume("file123", "example.zip", "https://example.com/file.zip", "./downloads/example.zip")

// 恢复下载
err := utils.ResumeDownloadByID("file123")

// 获取下载进度
progress := utils.GetDownloadProgressByID("file123")

// 列出所有下载
downloads := utils.ListAllDownloads()

// 取消下载
err := utils.CancelDownloadByID("file123")
```

### 2. 命令行工具

#### 基本用法
```bash
# 下载文件
go run cmd/download/main.go -url https://example.com/file.zip -name file.zip

# 指定输出路径
go run cmd/download/main.go -url https://example.com/file.zip -name file.zip -output ./downloads/

# 恢复下载
go run cmd/download/main.go -resume -id abc123_file.zip

# 列出所有下载
go run cmd/download/main.go -list

# 查看下载进度
go run cmd/download/main.go -progress -id abc123_file.zip

# 取消下载
go run cmd/download/main.go -cancel -id abc123_file.zip
```

#### 交互式CLI
```bash
# 启动交互式命令行界面
go run cmd/download/main.go -cli
```

在CLI中可以使用以下命令：
- `download <URL> <文件名> [保存路径]` - 开始新的下载
- `resume <文件ID>` - 恢复指定下载
- `pause <文件ID>` - 暂停指定下载
- `cancel <文件ID>` - 取消指定下载
- `list` - 列出所有下载
- `progress <文件ID>` - 查看下载进度
- `resume-all` - 恢复所有未完成的下载
- `help` - 显示帮助信息
- `quit` - 退出程序

### 3. 在现有项目中集成

在你的控制器中使用断点续传功能：

```go
// 在文件控制器中
func (fc *FileController) DownloadWithResume(c *gin.Context) {
    fileID := c.Param("id")
    fileName := c.Query("name")
    downloadURL := c.Query("url")
    
    // 生成唯一的下载ID
    downloadID := utils.GenerateID() + "_" + fileName
    
    // 开始下载
    err := utils.DownloadWithResume(downloadID, fileName, downloadURL, "./downloads/"+fileName)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, gin.H{
        "message": "Download started",
        "download_id": downloadID,
    })
}

func (fc *FileController) GetDownloadProgress(c *gin.Context) {
    downloadID := c.Param("download_id")
    
    progress := utils.GetDownloadProgressByID(downloadID)
    
    c.JSON(200, gin.H{
        "download_id": downloadID,
        "progress": progress,
    })
}
```

## 配置选项

### 状态文件存储位置
默认情况下，下载状态文件保存在用户主目录下的 `.downloads/states/` 目录中。你可以通过以下方式自定义：

```go
// 自定义状态目录
stateDir := "/path/to/custom/states"
downloader := utils.NewResumableDownloader(stateDir)
```

### 分块大小
默认分块大小为1MB，你可以根据网络条件调整：

```go
// 使用2MB分块
chunkSize := int64(2 * 1024 * 1024)
downloader.StartDownload(fileID, fileName, url, savePath, chunkSize)
```

### 并发数量
下载器默认使用5个并发连接，你可以在 `resume_download.go` 中修改：

```go
semaphore := make(chan struct{}, 10) // 改为10个并发连接
```

## 状态文件格式

下载状态以JSON格式保存，示例：

```json
{
  "file_id": "abc123_example.zip",
  "file_name": "example.zip",
  "file_path": "./downloads/example.zip",
  "file_size": 104857600,
  "download_url": "https://example.com/file.zip",
  "downloaded": 52428800,
  "status": "downloading",
  "created_at": "2024-01-01T10:00:00Z",
  "updated_at": "2024-01-01T10:05:00Z",
  "md5_hash": "",
  "chunk_size": 1048576,
  "chunks": [
    {
      "index": 0,
      "start": 0,
      "end": 1048575,
      "downloaded": true
    },
    {
      "index": 1,
      "start": 1048576,
      "end": 2097151,
      "downloaded": false
    }
  ]
}
```

## 错误处理

系统包含完善的错误处理机制：

1. **网络错误**: 自动重试，最多重试3次
2. **文件系统错误**: 详细的错误日志和用户友好的错误信息
3. **状态文件损坏**: 自动跳过损坏的状态文件
4. **磁盘空间不足**: 在下载前检查可用空间

## 性能优化

1. **并发下载**: 使用多个goroutine并发下载不同分块
2. **内存管理**: 使用缓冲池减少内存分配
3. **I/O优化**: 使用WriteAt进行随机写入，避免文件锁定
4. **网络优化**: 支持HTTP Range请求，减少不必要的数据传输

## 安全考虑

1. **路径验证**: 防止路径遍历攻击
2. **文件权限**: 创建的文件使用安全的权限设置
3. **URL验证**: 验证下载URL的合法性
4. **文件大小限制**: 可以设置最大文件大小限制

## 故障排除

### 常见问题

1. **下载速度慢**
   - 增加并发连接数
   - 调整分块大小
   - 检查网络连接

2. **下载失败**
   - 检查URL是否有效
   - 确认服务器支持Range请求
   - 查看错误日志

3. **状态文件损坏**
   - 删除损坏的状态文件
   - 重新开始下载

4. **磁盘空间不足**
   - 清理磁盘空间
   - 更改下载目录

### 日志查看

程序使用 `go-logger` 记录详细日志，可以通过以下方式查看：

```go
// 设置日志级别
logger.SetLevel(logger.DEBUG)
```

## 扩展功能

你可以基于现有代码扩展以下功能：

1. **下载队列**: 实现下载任务队列管理
2. **带宽限制**: 添加下载速度限制
3. **代理支持**: 添加HTTP代理支持
4. **加密下载**: 支持HTTPS和其他加密协议
5. **云存储**: 支持从云存储服务下载
6. **P2P下载**: 集成P2P下载功能

## 许可证

本项目采用MIT许可证，详见LICENSE文件。