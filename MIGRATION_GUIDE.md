# 代码模块化迁移指南

## 当前状态

已创建以下模块文件：
- ✅ `models.go` - 数据结构和全局变量
- ✅ `utils.go` - 工具函数
- ✅ `response.go` - HTTP响应函数

## 需要完成的工作

### 第一步：删除重复定义

由于 `ccr.go` 和 `aigo.go` 中已经定义了这些内容，需要删除以下重复定义：

#### 从 `aigo.go` 中删除：
- 所有数据结构定义（已移到 `models.go`）
- 所有全局变量定义（已移到 `models.go`）
- `getidai` 函数（应移到 `ai.go`）

#### 从 `ccr.go` 中删除：
- 所有工具函数（已移到 `utils.go`）
- 所有响应函数（已移到 `response.go`）
- 数据结构定义（已移到 `models.go`）

### 第二步：创建业务模块

需要创建以下业务模块文件，并将 `ccr.go` 中的对应函数迁移过去：

1. **bidding.go** - 招标管理
   - getBids, getBidDetail, getTodayBiddings, getBiddingsByDateRange
   - searchBids, insertBidHandler, insertChinaBidHandler, insertZhibiaoBidHandler
   - deleteBid, restoreBid, batchExclude, batchDelete
   - updateStatusHandler, updatePDFHandler
   - savePotentialBiddersHandler, getPotentialBiddersHandler
   - saveSimilarProjectsHandler, getSimilarProjectsHandler

2. **ai.go** - AI分析核心
   - analyzeBidHandler, analyzeByIDHandler, reanalyzeBidHandler
   - batchAnalyzeHandler, kimiAIHandler, chatWithAIHandler
   - callGLMAPI, callExternalAI, callDeepSeek, callExternalAIAPI
   - performBidAnalysis, runAIWorkflow
   - fetchBidRecordByID, fetchBidRecordBySerial, fetchBidRecordByField
   - fetchSystemPrompt, applyPromptTemplate, buildBidPrompt
   - parseAnalysisJSON, parseAIResponse
   - batchAnalyzeUnprocessedBids

3. **ai_scheduled.go** - AI定时任务
   - getAITasks, createAITask, updateAITask, deleteAITask
   - toggleAITask, executeAITask, getAITaskHistory
   - runScheduledTask, checkAndExecuteAITasks, shouldExecuteTask
   - fetchBiddingsForTask, callAIForScheduledTask
   - createTaskHistory, updateTaskHistorySuccess, updateTaskHistoryError
   - calculateNextRun, calculateNextRunFromNow, generateCronExpression
   - initAIRolesTable, initAIModelsTable

4. **tracking.go** - 追踪中标
   - trackWinnerHandler, startTrackedWinnerFetch
   - getTrackedBids, deleteTrackedBid, excludeAllBids
   - fetchTrackedBidsWinners, fetchTrackedBidWinner
   - parseWinnerInfo, parseCandidatesFromContent
   - downloadPDFToLocal, getLocalPDFPath, updatePDFURLInJSON
   - sendWinnerWechatNotification, formatWinnerInfoForWechat
   - ensureTrackedBidsEnhancements

5. **storage.go** - 文件存储
   - uploadChinaHandler, uploadChinaByIDHandler
   - handleFileUpload, handleFileGet, handleRawPutUpload
   - saveToStorage, tryServeStoredFile
   - pdfProxy

6. **wechat.go** - 微信通知
   - sendWechatNotification, sendProjectToWechatHandler
   - postWechatMessage, formatProjectForWechatFromMap
   - sendWechatAfterSingleAnalysis

7. **config.go** - 配置管理
   - getConfig, saveConfig
   - getAIRoles, createAIRole, updateAIRole, deleteAIRole
   - getAIModels, createAIModel, updateAIModel, deleteAIModel
   - getStatistics

8. **company.go** - 公司查询（已存在）
   - searchCompanyRealtime, getCompanyAwardSummary, getCompanyAwardRecords

### 第三步：更新 main.go

`main.go` 应该只保留：
- 数据库初始化
- 路由注册
- 定时任务启动
- 服务器启动

## 注意事项

1. 所有模块都在 `package main` 下，可以共享全局变量
2. 确保所有导入语句正确
3. 迁移后测试每个功能是否正常
4. 可以保留 `ccr.go` 作为备份，确认一切正常后再删除

## 建议的迁移顺序

1. 先迁移工具函数和响应函数（已完成）
2. 迁移配置管理模块（config.go）
3. 迁移招标管理模块（bidding.go）
4. 迁移AI分析模块（ai.go）
5. 迁移AI定时任务模块（ai_scheduled.go）
6. 迁移追踪模块（tracking.go）
7. 迁移存储模块（storage.go）
8. 迁移微信模块（wechat.go）
9. 最后更新 main.go

