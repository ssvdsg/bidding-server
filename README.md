# Bidding Server

招标信息智能分析系统 — 自动采集、分析招标公告，AI 辅助投标决策，微信实时通知。

## 功能

- 📊 **招标管理** — 多源采集、全文搜索、智能筛选、排除规则
- 🤖 **AI 分析** — 接入 DeepSeek V4，自动评估项目匹配度、生成投标建议
- 📈 **数据看板** — 可视化仪表盘，趋势分析、高分项目一览
- 🔔 **微信通知** — 高分项目实时推送、每日汇总报告
- 🏢 **公司档案** — 中标历史查询、竞争对手分析
- 📄 **PDF 预览** — 在线查看招标文件
- ⏰ **定时任务** — 自动同步、AI 批量分析、过期清理

## 技术栈

| 层 | 技术 |
|---|------|
| 后端 | Go 1.21 + Gorilla Mux |
| 数据库 | MySQL |
| 前端 | React 19 + TypeScript + Vite |
| UI 库 | Ant Design 6 + Recharts |
| 状态管理 | Zustand |
| AI | CTYun / Relay AI (DeepSeek V4) |
| 通知 | 微信 Hook |

## 快速开始

### 环境要求

- Go 1.21+
- Node.js 20+（推荐 22）
- MySQL 5.7+

### 1. 配置环境变量

```bash
cp .env.example .env
# 编辑 .env 填入数据库、AI、微信等配置
```

### 2. 构建 & 运行

```bash
# 一键构建前后端并启动
bash build_run.sh
```

或分步执行：

```bash
# 前端
cd frontend
npm install
npm run build

# 后端
go build -o bidding-server .
./bidding-server
```

服务默认监听 `0.0.0.0:3000`。

### 3. 开发模式

```bash
# 前端热重载
cd frontend && npm run dev

# 后端
go run .
```

## 项目结构

```
├── main.go              # 入口、路由、初始化
├── models.go            # 数据结构 & 常量
├── config.go            # 系统配置管理
├── ai.go                # AI 分析核心
├── ai_scheduled.go      # AI 定时任务
├── ai_client.go         # AI API 客户端
├── ai_prompt.md         # AI 分析提示词模板
├── bidding.go           # 招标管理
├── tracking.go          # 中标追踪
├── company_search.go    # 公司搜索
├── wechat.go            # 微信通知
├── storage.go           # 文件存储
├── utils.go             # 工具函数
├── response.go          # HTTP 响应
├── ccr.go               # 爬虫引擎
├── ctyun.go             # CTYun 直连
├── aigo.go              # AI 调度
├── user_profile.go      # 用户画像
├── frontend/            # React 前端
│   └── src/
│       ├── pages/       # 页面组件
│       ├── components/  # 通用组件
│       ├── api/         # API 客户端
│       ├── hooks/       # 自定义 Hooks
│       └── store/       # Zustand 状态
└── storage/r2/          # 本地缓存 (gitignored)
```

## 环境变量参考

完整配置项见 `.env.example`，主要包括：

| 变量 | 说明 |
|------|------|
| `DB_HOST` / `DB_PORT` / `DB_USER` / `DB_PASSWORD` / `DB_NAME` | 数据库连接 |
| `RELAY_AI_BASE_URL` / `RELAY_AI_API_KEY` / `RELAY_AI_MODEL` | AI 服务 |
| `WECHAT_HOOK_URL` / `WECHAT_NOTICE_BASE_URL` / `WECHAT_DEFAULT_ROOM` | 微信通知 |
| `PORT` | 服务端口 (默认 3000) |

## License

Private — 内部使用
