#!/bin/bash

# GoFileShare 启动脚本

echo "正在启动 GoFileShare 文件共享服务..."

# 检查是否已编译
if [ ! -f "./gofileshare" ]; then
    echo "未找到可执行文件，正在编译..."
    go build -o gofileshare .
    if [ $? -ne 0 ]; then
        echo "编译失败！"
        exit 1
    fi
    echo "编译完成！"
fi

# 启动服务
./gofileshare
