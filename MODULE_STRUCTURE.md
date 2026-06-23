# 模块化结构说明

## 文件组织

### 核心模块
- `models.go` - 数据结构和全局变量定义
- `utils.go` - 工具函数（字符串处理、编码解码、数据转换等）
- `response.go` - HTTP响应辅助函数
- `database.go` - 数据库查询辅助函数

### 业务模块
- `bidding.go` - 招标管理（查询、插入、删除、更新、搜索）
- `ai.go` - AI分析核心功能（AI调用、分析处理、批量分析）
- `ai_scheduled.go` - AI定时任务管理
- `tracking.go` - 追踪中标功能
- `storage.go` - 文件存储和上传
- `wechat.go` - 微信通知功能
- `config.go` - 配置管理（系统配置、AI角色、AI模型）
- `company.go` - 公司中标历史查询

### 入口文件
- `main.go` - 主程序入口（路由、初始化、定时任务）

## 迁移步骤

1. ✅ 已创建 `models.go` - 数据结构和全局变量
2. ✅ 已创建 `utils.go` - 工具函数
3. ✅ 已创建 `response.go` - HTTP响应函数
4. ⏳ 需要从 `ccr.go` 和 `aigo.go` 中移除重复定义
5. ⏳ 创建各个业务模块文件
6. ⏳ 更新 `main.go` 只保留路由和初始化

## 注意事项

- 所有模块都在 `package main` 下
- 全局变量在 `models.go` 中定义
- 工具函数在 `utils.go` 中定义
- 业务逻辑按功能拆分到对应模块

