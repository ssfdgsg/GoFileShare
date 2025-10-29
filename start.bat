@echo off
chcp 65001 >nul
echo ==========================================
echo 正在启动 GoFileShare 文件共享服务...
echo ==========================================

REM 检查是否已编译
if not exist "gofileshare.exe" (
    echo 未找到可执行文件，正在编译...
    go build -o gofileshare.exe .
    if errorlevel 1 (
        echo 编译失败！
        pause
        exit /b 1
    )
    echo 编译完成！
)

REM 启动服务
gofileshare.exe
pause
