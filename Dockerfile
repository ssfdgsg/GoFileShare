# ---- Builder Stage ----
# 使用一个带有完整 Go 工具链的镜像作为构建器
FROM golang:1.24 AS builder

# 设置工作目录
WORKDIR /app

# 1. 优先复制 go.mod 和 go.sum
# 这是为了利用 Docker 的层缓存。只要这两个文件不变，下面的下载步骤就不需要重新执行。
COPY go.mod go.sum ./

# 2. 下载依赖
RUN go mod download

# 3. 复制所有源代码
# 这是正确的写法，复制构建上下文中的所有文件到工作目录
COPY . .

# 4. 构建 Go 应用
# CGO_ENABLED=0: 禁用 CGO，生成静态链接的二进制文件，不依赖 C 库
# GOOS=linux: 指定目标操作系统为 Linux，因为我们的最终镜像是 alpine
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# (可选但推荐) 可以验证一下文件是否已生成
# RUN ls -l main

# ---- Final Stage ----
# 使用一个非常小的基础镜像，比如 alpine，来减小最终镜像的体积
FROM alpine:latest

# 设置工作目录
WORKDIR /app

# 从 builder 阶段复制编译好的二进制文件
COPY --from=builder /app/main .

COPY --from=builder /app/views ./views
# 设置容器启动时执行的命令
ENTRYPOINT ["./main"]
