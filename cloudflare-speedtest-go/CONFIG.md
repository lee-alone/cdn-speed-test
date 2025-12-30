# 配置文件说明

**版本**: 1.0.0  
**格式**: YAML

## 概述

程序使用`config.yaml`文件进行配置。如果文件不存在，程序会自动创建一份默认配置。

## 文件位置

配置文件位置：与二进制文件相同目录

```
bin/
├── cloudflare-speedtest.exe
└── config.yaml              ← 配置文件
```

## 自动创建

首次运行程序时，如果`config.yaml`不存在，程序会：
1. 使用默认配置值
2. 自动创建`config.yaml`文件
3. 后续运行时读取该文件

## 配置结构

### test - 测试设置

```yaml
test:
  expected_servers: 3        # 期待找到的服务数
  use_tls: false            # 是否使用TLS
  ip_type: ipv6             # IP类型: ipv4 或 ipv6
  bandwidth: 100            # 期望带宽 (Mbps)
  timeout: 5                # 超时时间 (秒)
  download_time: 10         # 下载测试时间 (秒)
  file_path: ./             # 结果保存路径
```

#### 参数说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| expected_servers | 整数 | 3 | 需要找到的合格IP数量 |
| use_tls | 布尔 | false | 是否使用TLS端口 |
| ip_type | 字符串 | ipv6 | IP类型选择 |
| bandwidth | 浮点数 | 100 | 最小带宽要求 |
| timeout | 整数 | 5 | 单个测试超时时间 |
| download_time | 整数 | 10 | 速度测试持续时间 |
| file_path | 字符串 | ./ | 结果文件保存目录 |

### download - 下载设置

```yaml
download:
  urls:
    ips-v4.txt: https://www.baipiao.eu.org/cloudflare/ips-v4
    ips-v6.txt: https://www.baipiao.eu.org/cloudflare/ips-v6
    colo.txt: https://www.baipiao.eu.org/cloudflare/colo
    url.txt: https://www.baipiao.eu.org/cloudflare/url
```

#### 参数说明

| 文件 | 说明 |
|------|------|
| ips-v4.txt | IPv4地址列表 |
| ips-v6.txt | IPv6地址列表 |
| colo.txt | 数据中心信息 |
| url.txt | 测速URL列表 |

## 默认配置

如果配置文件中缺少某些值，程序会使用以下默认值：

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

## 配置示例

### 示例1：基础配置

```yaml
test:
  expected_servers: 5
  use_tls: true
  ip_type: ipv4
  bandwidth: 50
  timeout: 10
  download_time: 15
  file_path: ./results/

download:
  urls:
    ips-v4.txt: https://www.baipiao.eu.org/cloudflare/ips-v4
    ips-v6.txt: https://www.baipiao.eu.org/cloudflare/ips-v6
    colo.txt: https://www.baipiao.eu.org/cloudflare/colo
    url.txt: https://www.baipiao.eu.org/cloudflare/url
```

### 示例2：自定义下载源

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
    ips-v4.txt: https://custom-source.com/ips-v4.txt
    ips-v6.txt: https://custom-source.com/ips-v6.txt
    colo.txt: https://custom-source.com/colo.txt
    url.txt: https://custom-source.com/url.txt
```

## 修改配置

### 方式1：直接编辑文件

1. 打开`config.yaml`文件
2. 修改所需参数
3. 保存文件
4. 重启程序

### 方式2：通过Web UI

1. 打开Web界面
2. 修改配置选项
3. 点击保存（如果实现）

## 配置验证

程序启动时会：
1. 检查配置文件是否存在
2. 如果不存在，创建默认配置
3. 如果存在，读取并验证
4. 缺失的值使用默认值填充

## 常见问题

### Q: 配置文件损坏了怎么办？
A: 删除`config.yaml`，程序会自动创建新的默认配置。

### Q: 如何恢复默认配置？
A: 
1. 删除`config.yaml`
2. 重启程序
3. 程序会创建新的默认配置

### Q: 可以有多个配置文件吗？
A: 目前不支持，但可以通过修改源代码实现。

### Q: 配置文件的编码是什么？
A: UTF-8编码

### Q: 如何添加新的下载源？
A: 在`download.urls`中添加新的条目：
```yaml
download:
  urls:
    new-file.txt: https://example.com/new-file.txt
```

## 配置文件格式

### YAML语法规则

1. **缩进**: 使用空格（不是Tab）
2. **冒号**: 键值对用冒号分隔，冒号后需要空格
3. **列表**: 使用`-`表示列表项
4. **字符串**: 可以不加引号，特殊字符需要引号
5. **布尔值**: `true`或`false`（小写）

### 有效的YAML示例

```yaml
# 字符串
name: value

# 数字
count: 42
ratio: 3.14

# 布尔值
enabled: true
disabled: false

# 列表
items:
  - item1
  - item2

# 嵌套对象
parent:
  child: value
```

## 性能建议

1. **bandwidth**: 根据实际网络情况调整
2. **timeout**: 网络不稳定时增加
3. **download_time**: 更长的时间获得更准确的结果
4. **expected_servers**: 根据需要调整

## 安全建议

1. 不要在配置文件中存储敏感信息
2. 定期备份配置文件
3. 在修改前备份原配置

## 更新日志

### v1.0.0
- 初始版本
- 支持YAML格式
- 自动创建默认配置
- 支持自定义下载源

---

**更多帮助**: 查看 [README.md](README.md) 或 [RUN.md](RUN.md)
