# 项目大小统计工具

## 概述

该工具用于统计项目的各种指标，包括文件数量、代码行数、文件大小等。

## 功能特性

- 📊 统计项目总文件数和目录数
- 📏 计算项目总大小（字节）
- 📝 统计代码行数（总行数、代码行数、空白行数）
- 🗂️ 按文件类型分类统计
- 🚫 自动排除常见的非代码目录（.git, .idea, node_modules等）
- 🚫 自动排除二进制文件和媒体文件

## 使用方法

### 方法1: 使用命令行工具

```bash
# 编译命令行工具
go build -o project-size cmd_project_size.go

# 统计当前目录
./project-size

# 统计指定目录
./project-size /path/to/your/project
```

### 方法2: 在代码中使用

```go
package main

import (
    "GoFileShare/utils"
    "fmt"
)

func main() {
    // 统计项目大小
    stats, err := utils.CalculateProjectSize(".")
    if err != nil {
        fmt.Printf("错误: %v\n", err)
        return
    }
    
    // stats 包含详细的统计信息
    fmt.Printf("总文件数: %d\n", stats.TotalFiles)
    fmt.Printf("总大小: %d bytes\n", stats.TotalSize)
    fmt.Printf("总行数: %d\n", stats.TotalLines)
}
```

### 方法3: 使用特定函数

```go
package main

import (
    "GoFileShare/utils"
    "fmt"
)

func main() {
    // 只获取目录大小
    size, err := utils.GetDirectorySize(".")
    if err != nil {
        fmt.Printf("错误: %v\n", err)
        return
    }
    fmt.Printf("目录大小: %d bytes\n", size)
    
    // 只获取文件数量
    count, err := utils.GetFileCount(".")
    if err != nil {
        fmt.Printf("错误: %v\n", err)
        return
    }
    fmt.Printf("文件数量: %d\n", count)
    
    // 统计特定扩展名的代码行数
    lines, err := utils.GetLinesOfCode(".", []string{".go", ".js", ".py"})
    if err != nil {
        fmt.Printf("错误: %v\n", err)
        return
    }
    fmt.Printf("代码行数: %d\n", lines)
}
```

## API 参考

### ProjectStats 结构

```go
type ProjectStats struct {
    TotalFiles      int                       // 总文件数
    TotalDirs       int                       // 总目录数
    TotalSize       int64                     // 总大小（字节）
    TotalLines      int64                     // 总行数
    TotalCodeLines  int64                     // 代码行数
    TotalBlankLines int64                     // 空白行数
    FileTypeStats   map[string]*FileTypeStats // 按文件类型统计
}
```

### FileTypeStats 结构

```go
type FileTypeStats struct {
    Count      int   // 文件数量
    TotalSize  int64 // 总大小（字节）
    TotalLines int64 // 总行数
}
```

### 主要函数

#### CalculateProjectSize

统计项目的完整信息并打印到控制台。

```go
func CalculateProjectSize(rootPath string) (*ProjectStats, error)
```

**参数:**
- `rootPath`: 项目根目录路径

**返回:**
- `*ProjectStats`: 项目统计信息
- `error`: 错误信息

#### GetDirectorySize

获取目录的总大小。

```go
func GetDirectorySize(dirPath string) (int64, error)
```

**参数:**
- `dirPath`: 目录路径

**返回:**
- `int64`: 目录大小（字节）
- `error`: 错误信息

#### GetFileCount

获取目录中的文件数量。

```go
func GetFileCount(dirPath string) (int, error)
```

**参数:**
- `dirPath`: 目录路径

**返回:**
- `int`: 文件数量
- `error`: 错误信息

#### GetLinesOfCode

获取指定扩展名文件的代码行数。

```go
func GetLinesOfCode(dirPath string, extensions []string) (int64, error)
```

**参数:**
- `dirPath`: 目录路径
- `extensions`: 文件扩展名列表（如 []string{".go", ".js"}）

**返回:**
- `int64`: 代码行数
- `error`: 错误信息

#### CountLinesInFile

统计单个文件的行数。

```go
func CountLinesInFile(filePath string) (int64, error)
```

**参数:**
- `filePath`: 文件路径

**返回:**
- `int64`: 行数
- `error`: 错误信息

## 输出示例

```
========== 项目统计信息 ==========
项目路径: /home/user/project
总文件数: 150
总目录数: 25
总大小: 2.45 MB
总行数: 12500
代码行数: 10200
空白行数: 2300

========== 文件类型统计 ==========
.go: 45 文件, 1.23 MB, 8500 行
.js: 30 文件, 856.00 KB, 2800 行
.md: 10 文件, 125.50 KB, 1200 行
.json: 15 文件, 78.20 KB, 0 行
==================================
```

## 排除的目录

以下目录会自动被排除在统计之外：
- `.git`
- `.idea`
- `.vscode`
- `node_modules`
- `vendor`
- `.DS_Store`

## 排除的文件类型

以下文件类型会被排除在行数统计之外（但会统计在大小中）：
- 可执行文件: `.exe`, `.dll`, `.so`, `.dylib`, `.bin`
- 数据库文件: `.dat`, `.db`, `.sqlite`
- 图片文件: `.jpg`, `.jpeg`, `.png`, `.gif`, `.bmp`, `.ico`, `.svg`
- 视频文件: `.mp4`, `.avi`, `.mov`, `.wmv`
- 音频文件: `.mp3`, `.wav`
- 压缩文件: `.zip`, `.tar`, `.gz`, `.rar`, `.7z`

## 性能优化

- 使用 `bufio.Scanner` 进行高效的文件读取
- 使用 `sync.Mutex` 确保并发安全
- 大文件处理使用 1MB 缓冲区
- 自动跳过二进制和媒体文件的行数统计

## 注意事项

1. 统计大型项目可能需要一些时间
2. 确保有足够的权限读取目标目录
3. 符号链接会被正常处理
4. 对于无法访问的文件或目录，会记录警告但继续执行

## 扩展

如果需要自定义排除规则，可以创建 `ProjectSizeCalculator` 实例并修改 `excludeDirs` 和 `excludeExts` 字段：

```go
calculator := utils.NewProjectSizeCalculator(".")
calculator.excludeDirs[".custom"] = true
calculator.excludeExts[".custom"] = true
stats, err := calculator.Calculate()
if err != nil {
    // 处理错误
}
calculator.PrintStats()
```
