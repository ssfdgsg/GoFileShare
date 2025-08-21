@echo off
setlocal

:: 获取开始时间
for /f "tokens=1-4 delims=:.," %%a in ("%time%") do (
    set /a "start=(((%%a*60)+%%b)*60+%%c)*100+%%d"
)

:: 执行你想要测量时间的命令
cmd /c "go fmt ."

:: 获取结束时间
for /f "tokens=1-4 delims=:.," %%a in ("%time%") do (
    set /a "end=(((%%a*60)+%%b)*60+%%c)*100+%%d"
)

:: 计算并显示执行时间
set /a "duration=(end-start)"
echo fmt cost time:%duration% ms

for /f "tokens=1-4 delims=:.," %%a in ("%time%") do (
    set /a "start=(((%%a*60)+%%b)*60+%%c)*100+%%d"
)

cmd /c "go build main.go"

for /f "tokens=1-4 delims=:.," %%a in ("%time%") do (
    set /a "end=(((%%a*60)+%%b)*60+%%c)*100+%%d"
)

set /a "duration=(end-start)"
echo build cost time:%duration% ms

endlocal