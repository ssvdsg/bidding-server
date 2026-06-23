<p align="center">
  <img src="frontend/public/favicon.svg" width="80" alt="Bidding Server" />
</p>

<h1 align="center">Bidding Server</h1>
<p align="center">
  招标信息智能分析系统 — AI 驱动的投标决策助手
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21-00ADD8?style=flat-square&logo=go" alt="Go" />
  <img src="https://img.shields.io/badge/React-19-61DAFB?style=flat-square&logo=react" alt="React" />
  <img src="https://img.shields.io/badge/TypeScript-6.0-3178C6?style=flat-square&logo=typescript" alt="TS" />
  <img src="https://img.shields.io/badge/Vite-8.0-646CFF?style=flat-square&logo=vite" alt="Vite" />
  <img src="https://img.shields.io/badge/Ant_Design-6.0-0170FE?style=flat-square&logo=antdesign" alt="AntD" />
  <img src="https://img.shields.io/badge/AI-DeepSeek_V4-6366F1?style=flat-square" alt="AI" />
  <img src="https://img.shields.io/badge/DB-MySQL-4479A1?style=flat-square&logo=mysql" alt="MySQL" />
</p>

---

## ✨ 功能亮点

<table>
  <tr>
    <td width="50%">
      <h3>🤖 AI 智能分析</h3>
      <p>接入 <b>DeepSeek V4</b> 大模型，自动分析招标项目与公司业务的匹配度，生成多维度评分（地域匹配、行业相关、规模适配），输出结构化投标建议。</p>
    </td>
    <td width="50%">
      <h3>📊 数据可视化</h3>
      <p>交互式仪表盘，支持招标趋势、高分项目分布、行业热力图等图表，一目了然掌握市场动态。</p>
    </td>
  </tr>
  <tr>
    <td>
      <h3>🔍 多源采集</h3>
      <p>支持多个主流招标公告平台自动抓取，去重校验，定时同步。</p>
    </td>
    <td>
      <h3>🔔 微信实时通知</h3>
      <p>高分项目自动推送到微信群，支持每日汇总报告，阈值可配置。</p>
    </td>
  </tr>
  <tr>
    <td>
      <h3>🏢 公司档案</h3>
      <p>查询竞争对手中标历史、中标金额、中标趋势，辅助制定投标策略。</p>
    </td>
    <td>
      <h3>📄 在线预览</h3>
      <p>内置 PDF.js 渲染引擎，无需下载即可在线查看招标文件全文。</p>
    </td>
  </tr>
  <tr>
    <td>
      <h3>⏰ 定时任务</h3>
      <p>自动同步新公告、AI 批量分析、过期项目自动排除、数据清理，全自动化运行。</p>
    </td>
    <td>
      <h3>🎯 智能筛选</h3>
      <p>关键词匹配、状态标签、行业过滤、高分/低分标记，快速定位值得投标的项目。</p>
    </td>
  </tr>
</table>

## 🏗️ 系统架构

```
┌──────────────────────┐
│    go-Zcaiji 采集端   │  ← 浏览器插件 / 自动采集
│  多平台招标公告抓取    │    去重 → 推送 → 本服务
└──────────┬───────────┘
           │ POST /api/insertChinaBid
           ▼
┌──────────────────────────────────────────────────────┐
│                     Frontend                         │
│         React 19 + TypeScript + Vite 8               │
│         Ant Design 6 · Recharts · Zustand            │
└──────────────────────┬───────────────────────────────┘
                       │ REST API
┌──────────────────────┴───────────────────────────────┐
│                     Backend                          │
│              Go 1.21 + Gorilla Mux                   │
│                                                      │
│   ┌──────────┐ ┌──────────┐ ┌──────────────┐        │
│   │ 招标管理  │ │ AI 分析   │ │ 定时任务调度  │        │
│   └──────────┘ └──────────┘ └──────────────┘        │
│   ┌──────────┐ ┌──────────┐ ┌──────────────┐        │
│   │ 公司搜索  │ │ 微信通知  │ │ 文件存储(R2)  │        │
│   └──────────┘ └──────────┘ └──────────────┘        │
└──────────────────────┬───────────────────────────────┘
                       │
┌──────────────────────┴───────────────────────────────┐
│                  MySQL · AI 服务                     │
│                   WeChat Hook                        │
└──────────────────────────────────────────────────────┘
```

> 📡 **采集端独立部署**：[go-Zcaiji](https://github.com/ssvdsg/go-Zcaiji) — 负责多平台招标公告自动抓取、去重后通过 API 推送至本服务。

## 📁 项目结构

```
bidding-server/
├── main.go                 # 入口 · 路由注册 · 定时任务
├── models.go               # 数据结构 & 全局常量
├── config.go               # 系统配置 CRUD · AI 模型同步
├── ai.go                   # AI 分析引擎 · 提示词拼装
├── ai_scheduled.go         # 定时 AI 任务管理
├── ai_client.go            # AI API 客户端（流式/非流式）
├── ai_prompt.md            # 分析提示词模板
├── bidding.go              # 招标 CRUD · 搜索 · 排除
├── tracking.go             # 中标追踪 · PDF 下载
├── company_search.go       # 公司中标历史查询
├── wechat.go               # 微信消息推送
├── storage.go              # 文件上传存储
├── ccr.go                  # 数据采集引擎
├── ctyun.go                # CTYun 直连通道
├── utils.go                # 工具函数
├── response.go             # 统一 HTTP 响应格式
├── frontend/               # React SPA
│   └── src/
│       ├── pages/
│       │   ├── Dashboard/      # 数据看板
│       │   ├── Bids/           # 招标列表 · 详情 · 分析
│       │   ├── Companies/      # 公司档案 · 中标记录
│       │   ├── Chat/           # AI 对话
│       │   ├── Tracked/        # 追踪项目
│       │   └── Settings/       # 系统设置 · AI 任务
│       ├── components/         # 通用组件
│       ├── api/                # API 客户端封装
│       ├── hooks/              # 自定义 Hooks
│       └── store/              # Zustand 状态管理
└── build_run.sh            # 一键构建启动脚本
```

## 🚀 快速开始

### 环境要求

| 工具 | 版本 |
|------|------|
| Go | ≥ 1.21 |
| Node.js | ≥ 20（推荐 22） |
| MySQL | ≥ 5.7 |

### 1. 克隆 & 配置

```bash
git clone https://github.com/ssvdsg/bidding-server.git
cd bidding-server
cp .env.example .env
# 编辑 .env 填入数据库、AI、微信等配置
```

### 2. 一键启动

```bash
bash build_run.sh
```

### 3. 开发模式

```bash
# 终端 1：前端热重载
cd frontend && npm install && npm run dev

# 终端 2：后端
go run .
```

> 访问 http://localhost:5173 进入开发模式（API 自动代理到 3000 端口）

## ⚙️ 环境变量

> **完整配置模板见 [`.env.example`](.env.example)**

```bash
# 数据库
DB_HOST=     DB_PORT=     DB_USER=     DB_PASSWORD=     DB_NAME=

# AI 服务
RELAY_AI_BASE_URL=    RELAY_AI_API_KEY=    RELAY_AI_MODEL=

# 微信通知
WECHAT_HOOK_URL=      WECHAT_NOTICE_BASE_URL=    WECHAT_DEFAULT_ROOM=

# 服务端口（默认 3000）
PORT=
```

## 🖥️ 界面预览

| 仪表盘 | 招标列表 |
|:---:|:---:|
| ![Dashboard](frontend/src/assets/hero.png) | 数据看板 + 趋势分析 |

| AI 分析 | 公司档案 |
|:---:|:---:|
| DeepSeek V4 智能评分 | 中标历史追踪 |

## 🔗 相关项目

| 项目 | 说明 |
|------|------|
| [go-Zcaiji](https://github.com/ssvdsg/go-Zcaiji) | 📡 采集端 — 多平台招标公告自动抓取，去重后推送至 bidding-server |

## 📄 License

MIT © 2025
