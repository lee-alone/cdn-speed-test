@echo off
setlocal enabledelayedexpansion

echo Building Cloudflare Speed Test...
echo.

REM 检查Go是否安装
go version >nul 2>&1
if errorlevel 1 (
    echo Error: Go is not installed or not in PATH
    pause
    exit /b 1
)

REM 设置编译环境变量
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0

REM 创建bin目录
if not exist "bin" mkdir bin

REM 编译（带优化）
go build -trimpath -ldflags="-s -w -X main.Version=1.0" -o bin\cloudflare-speedtest.exe .

if errorlevel 1 (
    echo Build failed!
    pause
    exit /b 1
)

echo Build completed: bin\cloudflare-speedtest.exe
pause
