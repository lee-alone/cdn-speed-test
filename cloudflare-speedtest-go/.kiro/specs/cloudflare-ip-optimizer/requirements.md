# Requirements Document

## Introduction

Cloudflare IP优选测速系统是一个用于测试和筛选最优Cloudflare IP节点的Web应用程序。系统通过测试不同的Cloudflare IP节点的延迟和下载速度，找到满足带宽要求的最优IP，并提供Web界面进行配置和监控。

## Glossary

- **System**: Cloudflare IP优选测速系统
- **IP_Generator**: IP地址生成器，用于从子网生成随机IP
- **Speed_Tester**: 速度测试器，执行延迟和速度测试
- **Config_Manager**: 配置管理器，管理系统配置参数
- **Data_Downloader**: 数据下载器，下载和管理配置文件
- **Web_Interface**: Web界面，提供用户交互功能
- **Test_Result**: 测试结果，包含IP、延迟、速度等信息
- **Data_Center**: 数据中心，Cloudflare节点的地理位置

## Requirements

### Requirement 1: 配置管理优化

**User Story:** 作为系统管理员，我希望能够完善配置管理功能，以便更好地控制测试参数和系统行为。

#### Acceptance Criteria

1. WHEN 用户访问Web界面 THEN THE System SHALL 显示所有可配置的参数选项
2. WHEN 用户修改配置参数 THEN THE System SHALL 验证参数的有效性并提供实时反馈
3. WHEN 用户保存配置 THEN THE System SHALL 将配置持久化到YAML文件并确认保存成功
4. WHEN 系统启动时 THEN THE System SHALL 加载配置文件或创建默认配置
5. WHERE 配置文件不存在或损坏 THEN THE System SHALL 使用默认配置并创建新的配置文件

### Requirement 2: 前端界面完善

**User Story:** 作为用户，我希望前端界面能够提供完整的配置选项和更好的用户体验，以便更方便地使用系统。

#### Acceptance Criteria

1. WHEN 用户访问主页面 THEN THE Web_Interface SHALL 显示所有配置选项的完整表单
2. WHEN 用户修改数据中心选择 THEN THE Web_Interface SHALL 提供数据中心下拉列表供选择
3. WHEN 测试进行中 THEN THE Web_Interface SHALL 实时显示测试进度和当前状态
4. WHEN 测试完成 THEN THE Web_Interface SHALL 显示详细的测试结果表格
5. WHEN 配置参数无效 THEN THE Web_Interface SHALL 显示错误提示并阻止提交

### Requirement 3: 数据中心筛选功能

**User Story:** 作为用户，我希望能够选择特定的数据中心进行测试，以便获得符合地理位置要求的IP。

#### Acceptance Criteria

1. WHEN 系统启动时 THEN THE System SHALL 加载数据中心映射文件
2. WHEN 用户选择数据中心 THEN THE System SHALL 只测试属于该数据中心的IP
3. WHERE 用户选择"全部数据中心" THEN THE System SHALL 测试所有可用的IP
4. WHEN IP测试时 THEN THE System SHALL 通过/cdn-cgi/trace端点获取数据中心信息
5. WHEN 显示结果时 THEN THE System SHALL 显示数据中心的友好名称而非代码

### Requirement 4: 并发测试优化

**User Story:** 作为用户，我希望系统能够支持并发测试多个IP，以便提高测试效率。

#### Acceptance Criteria

1. WHEN 开始测试时 THEN THE System SHALL 支持配置并发测试的IP数量
2. WHEN 并发测试进行中 THEN THE System SHALL 正确管理goroutine池避免资源耗尽
3. WHEN 并发测试时 THEN THE System SHALL 确保测试结果的线程安全性
4. WHEN 用户停止测试 THEN THE System SHALL 优雅地停止所有正在进行的测试
5. WHEN 并发测试完成 THEN THE System SHALL 按照速度或延迟对结果进行排序

### Requirement 5: 测试结果管理

**User Story:** 作为用户，我希望能够更好地管理和导出测试结果，以便进行后续分析和使用。

#### Acceptance Criteria

1. WHEN 测试完成 THEN THE System SHALL 将合格的IP结果保存到指定文件
2. WHEN 用户查看结果 THEN THE System SHALL 显示IP、延迟、速度、数据中心等详细信息
3. WHEN 用户导出结果 THEN THE System SHALL 支持多种格式导出（CSV、JSON、TXT）
4. WHEN 结果文件已存在 THEN THE System SHALL 检查重复IP并避免重复写入
5. WHEN 用户清空结果 THEN THE System SHALL 清除内存中的结果但保留已保存的文件

### Requirement 6: 错误处理和日志

**User Story:** 作为系统管理员，我希望系统能够提供完善的错误处理和日志记录，以便排查问题和监控系统状态。

#### Acceptance Criteria

1. WHEN 网络请求失败 THEN THE System SHALL 记录错误信息并继续测试其他IP
2. WHEN 配置文件读取失败 THEN THE System SHALL 使用默认配置并记录警告
3. WHEN 数据文件下载失败 THEN THE System SHALL 提供重试机制并显示错误状态
4. WHEN 系统运行时 THEN THE System SHALL 记录关键操作和性能指标到日志文件
5. IF 发生严重错误 THEN THE System SHALL 优雅地处理错误并提供用户友好的错误信息

### Requirement 7: 性能监控和统计

**User Story:** 作为用户，我希望能够查看测试的实时统计信息，以便了解测试进度和效果。

#### Acceptance Criteria

1. WHEN 测试进行中 THEN THE System SHALL 实时更新测试统计信息
2. WHEN 显示统计信息时 THEN THE System SHALL 包含总数、已完成、合格数量等指标
3. WHEN 测试IP时 THEN THE System SHALL 显示当前正在测试的IP地址
4. WHEN 计算平均速度时 THEN THE System SHALL 使用滑动窗口算法平滑速度波动
5. WHEN 测试完成时 THEN THE System SHALL 显示测试总耗时和效率统计

### Requirement 8: 数据文件管理

**User Story:** 作为系统管理员，我希望系统能够自动管理所需的数据文件，以便确保测试数据的及时性和完整性。

#### Acceptance Criteria

1. WHEN 系统启动时 THEN THE System SHALL 检查所需数据文件的存在性
2. WHEN 数据文件缺失 THEN THE System SHALL 自动从配置的URL下载缺失文件
3. WHEN 用户点击更新数据 THEN THE System SHALL 重新下载所有数据文件
4. WHEN 下载进行中 THEN THE System SHALL 显示下载进度和状态
5. WHEN 下载完成 THEN THE System SHALL 验证文件完整性并更新状态显示

### Requirement 9: IP生成和去重

**User Story:** 作为系统开发者，我希望IP生成器能够高效地生成随机IP并避免重复测试，以便提高测试效率。

#### Acceptance Criteria

1. WHEN 生成IP时 THEN THE IP_Generator SHALL 从配置的子网列表中随机选择子网
2. WHEN 生成IP时 THEN THE IP_Generator SHALL 为每个子网生成指定数量的随机IP
3. WHEN 检查重复时 THEN THE IP_Generator SHALL 维护已测试IP的集合避免重复
4. WHEN 子网耗尽时 THEN THE IP_Generator SHALL 智能地重新开始或扩展搜索范围
5. WHEN 支持IPv6时 THEN THE IP_Generator SHALL 正确处理IPv6地址格式和子网

### Requirement 10: 速度测试算法

**User Story:** 作为用户，我希望速度测试能够准确反映网络性能，以便选择真正优质的IP。

#### Acceptance Criteria

1. WHEN 测试延迟时 THEN THE Speed_Tester SHALL 发送请求到/cdn-cgi/trace端点并测量往返时间
2. WHEN 测试速度时 THEN THE Speed_Tester SHALL 下载指定文件并实时计算传输速度
3. WHEN 计算速度时 THEN THE Speed_Tester SHALL 使用定期采样和滑动窗口算法
4. WHEN 记录结果时 THEN THE Speed_Tester SHALL 保存平均速度、峰值速度和延迟信息
5. WHEN 测试超时时 THEN THE Speed_Tester SHALL 标记IP为超时并继续测试下一个IP