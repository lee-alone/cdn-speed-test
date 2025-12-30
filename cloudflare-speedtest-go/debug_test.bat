@echo off
echo === Cloudflare IP 测速调试脚本 ===
echo.

echo 1. 检查程序是否运行...
curl -s http://localhost:8081/api/status > nul
if %errorlevel% neq 0 (
    echo 错误: 程序未运行或端口8081不可用
    echo 请先启动程序
    pause
    exit /b 1
)
echo ✓ 程序正在运行

echo.
echo 2. 获取当前状态...
curl -s http://localhost:8081/api/status

echo.
echo 3. 获取调试信息...
curl -s http://localhost:8081/api/debug

echo.
echo 4. 检查配置...
curl -s http://localhost:8081/api/config

echo.
echo 5. 检查数据中心...
curl -s http://localhost:8081/api/datacenters

echo.
echo === 调试完成 ===
echo 如果测试卡住，请检查以上信息中的：
echo - testing 字段是否为 true
echo - stats 中的 Total 和 Completed 数值
echo - worker_stats 中的运行状态
echo - managers 中的加载状态
echo.
pause