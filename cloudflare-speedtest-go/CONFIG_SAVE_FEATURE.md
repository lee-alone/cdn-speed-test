# 配置保存功能说明

**日期**: 2025年12月30日  
**版本**: 1.0.0  
**功能**: 用户可以在Web UI中修改配置并保存到config.yaml文件

## 功能概述

用户现在可以：
1. 在Web UI中修改配置参数
2. 点击"保存配置"按钮
3. 配置会被保存到`config.yaml`文件
4. 程序重启时会读取保存的配置

## 工作流程

### 修改配置

1. 打开Web UI：`http://localhost:8080`
2. 在配置面板中修改参数：
   - IP类型
   - 期待服务数
   - 期望带宽
   - 超时时间
   - 下载时间
   - 启用TLS

### 保存配置

1. 修改完成后，点击"保存配置"按钮
2. 按钮会显示"保存中..."
3. 配置会被保存到`config.yaml`
4. 显示"配置已保存"提示

### 验证保存

1. 打开`config.yaml`文件
2. 检查修改的参数是否已保存
3. 重启程序验证配置是否生效

## API 端点

### POST /api/config
更新内存中的配置

**请求**:
```json
{
  "test": {
    "expected_servers": 3,
    "use_tls": false,
    "ip_type": "ipv6",
    "bandwidth": 100,
    "timeout": 5,
    "download_time": 10,
    "file_path": "./"
  },
  "download": {
    "urls": {}
  }
}
```

**响应**:
```json
{
  "message": "config updated in memory"
}
```

### POST /api/config/save
保存配置到文件

**请求**: 无请求体

**响应**:
```json
{
  "message": "config saved successfully"
}
```

## 技术实现

### 后端

#### server.go
- 添加`configPath`字段存储配置文件路径
- 实现`saveConfig()`函数保存配置到文件
- 修改`updateConfig()`函数只更新内存配置

#### main.go
- 获取配置文件路径
- 传递给`server.New()`函数

### 前端

#### index.html
- 添加"保存配置"按钮
- 实现`saveConfig()`JavaScript函数
- 收集表单数据并发送到API

## 配置保存流程

```
用户修改配置
    ↓
点击"保存配置"按钮
    ↓
JavaScript收集表单数据
    ↓
POST /api/config (更新内存)
    ↓
POST /api/config/save (保存到文件)
    ↓
显示成功提示
    ↓
config.yaml 已更新
```

## 文件位置

配置文件位置：
```
bin/
├── cloudflare-speedtest.exe
└── config.yaml              ← 保存的配置文件
```

## 配置文件格式

保存后的`config.yaml`格式：

```yaml
test:
  expected_servers: 3
  use_tls: false
  ip_type: ipv6
  bandwidth: 100
  timeout: 5
  download_time: 10
  file_path: ./

download:
  urls:
    ips-v4.txt: https://www.baipiao.eu.org/cloudflare/ips-v4
    ips-v6.txt: https://www.baipiao.eu.org/cloudflare/ips-v6
    colo.txt: https://www.baipiao.eu.org/cloudflare/colo
    url.txt: https://www.baipiao.eu.org/cloudflare/url
```

## 使用示例

### 示例1：修改期待服务数

1. 打开Web UI
2. 将"期待服务数"从3改为5
3. 点击"保存配置"
4. 配置已保存

### 示例2：修改IP类型

1. 打开Web UI
2. 将"IP类型"从IPv6改为IPv4
3. 点击"保存配置"
4. 配置已保存

### 示例3：启用TLS

1. 打开Web UI
2. 勾选"启用TLS"复选框
3. 点击"保存配置"
4. 配置已保存

## 错误处理

### 保存失败

如果保存失败，会显示错误提示：
```
保存失败: [错误信息]
```

常见原因：
- 文件权限不足
- 磁盘空间不足
- 文件被其他程序占用

### 解决方案

1. 检查文件权限
2. 检查磁盘空间
3. 关闭其他占用文件的程序
4. 重试保存

## 注意事项

1. **配置立即生效**: 修改后的配置在内存中立即生效
2. **持久化保存**: 点击"保存配置"才会保存到文件
3. **程序重启**: 程序重启时会读取保存的配置
4. **备份建议**: 修改前建议备份`config.yaml`

## 高级用法

### 手动编辑配置

1. 停止程序
2. 编辑`config.yaml`文件
3. 重启程序

### 恢复默认配置

1. 删除`config.yaml`
2. 重启程序
3. 程序会创建新的默认配置

### 导出配置

1. 复制`config.yaml`文件
2. 在其他位置使用

### 导入配置

1. 将`config.yaml`复制到程序目录
2. 重启程序

## 常见问题

### Q: 修改后需要重启程序吗？
A: 不需要。修改后的配置在内存中立即生效。但如果要在程序重启后保持配置，需要点击"保存配置"。

### Q: 保存配置后多久生效？
A: 立即生效。配置保存到文件后，程序重启时会读取新配置。

### Q: 可以同时修改多个参数吗？
A: 可以。修改完所有参数后，点击一次"保存配置"即可。

### Q: 如何验证配置是否保存成功？
A: 
1. 查看是否显示"配置已保存"提示
2. 打开`config.yaml`文件检查
3. 重启程序验证配置是否生效

### Q: 保存失败怎么办？
A: 
1. 检查错误提示
2. 检查文件权限
3. 检查磁盘空间
4. 重试保存

## 技术细节

### 配置更新流程

1. **前端收集数据**: JavaScript从表单收集用户输入
2. **发送到后端**: 通过POST请求发送到`/api/config`
3. **更新内存**: 服务器更新内存中的配置
4. **保存到文件**: 通过POST请求发送到`/api/config/save`
5. **写入文件**: 服务器将配置写入`config.yaml`

### 配置验证

- 所有参数都有默认值
- 缺失的参数使用默认值填充
- 无效的参数被忽略

## 性能考虑

- 配置保存是同步操作
- 大多数情况下保存时间<100ms
- 不会阻塞其他操作

## 安全考虑

- 配置文件权限为644（用户可读写，其他用户只读）
- 不建议在配置中存储敏感信息
- 定期备份配置文件

---

**更多帮助**: 查看 [CONFIG.md](CONFIG.md) 或 [README.md](README.md)
