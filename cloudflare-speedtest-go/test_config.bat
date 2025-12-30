@echo off
echo === 配置保存测试脚本 ===
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
echo 2. 获取当前配置...
curl -s -H "Content-Type: application/json" http://localhost:8081/api/config

echo.
echo 3. 测试配置验证...
curl -s -X POST -H "Content-Type: application/json" -d "{\"test\":{\"expected_servers\":3,\"use_tls\":false,\"ip_type\":\"ipv4\",\"bandwidth\":100,\"timeout\":5,\"download_time\":10,\"file_path\":\"./\",\"datacenter_filter\":\"all\",\"concurrent_workers\":10,\"sample_interval\":1},\"download\":{\"urls\":{\"ips-v4.txt\":\"https://www.baipiao.eu.org/cloudflare/ips-v4\",\"ips-v6.txt\":\"https://www.baipiao.eu.org/cloudflare/ips-v6\",\"colo.txt\":\"https://www.baipiao.eu.org/cloudflare/colo\",\"url.txt\":\"https://www.baipiao.eu.org/cloudflare/url\"}},\"ui\":{\"datacenter_filter\":\"all\",\"result_format\":\"table\",\"auto_refresh\":true,\"theme\":\"light\"},\"advanced\":{\"concurrent_workers\":10,\"retry_attempts\":3,\"log_level\":\"info\",\"enable_metrics\":true}}" http://localhost:8081/api/config/validate

echo.
echo 4. 测试配置更新...
curl -s -X POST -H "Content-Type: application/json" -d "{\"test\":{\"expected_servers\":3,\"use_tls\":false,\"ip_type\":\"ipv4\",\"bandwidth\":100,\"timeout\":5,\"download_time\":10,\"file_path\":\"./\",\"datacenter_filter\":\"all\",\"concurrent_workers\":10,\"sample_interval\":1},\"download\":{\"urls\":{\"ips-v4.txt\":\"https://www.baipiao.eu.org/cloudflare/ips-v4\",\"ips-v6.txt\":\"https://www.baipiao.eu.org/cloudflare/ips-v6\",\"colo.txt\":\"https://www.baipiao.eu.org/cloudflare/colo\",\"url.txt\":\"https://www.baipiao.eu.org/cloudflare/url\"}},\"ui\":{\"datacenter_filter\":\"all\",\"result_format\":\"table\",\"auto_refresh\":true,\"theme\":\"light\"},\"advanced\":{\"concurrent_workers\":10,\"retry_attempts\":3,\"log_level\":\"info\",\"enable_metrics\":true}}" http://localhost:8081/api/config

echo.
echo 5. 测试配置保存...
curl -s -X POST http://localhost:8081/api/config/save

echo.
echo === 测试完成 ===
echo 如果看到错误信息，请检查：
echo 1. 配置格式是否正确
echo 2. 字段名称是否匹配
echo 3. 数值范围是否有效
echo 4. 文件权限是否正确
echo.
pause