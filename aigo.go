// AI相关数据结构和全局变量定义文件
package main

import (
	"log"
)

// getidai 异步获取并分析指定ID的招标项目
// 参数:
//   - id: 招标项目的唯一标识符
func getidai(id string) {
	// 使用goroutine异步执行，避免阻塞主线程
	go func(id string) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[AI分析] 项目 %s 分析时发生panic: %v", id, r)
			}
		}()

		enabled, err := isAutoAIAnalysisEnabled()
		if err != nil {
			log.Printf("[AI分析] 读取自动评分开关失败: %v", err)
			return
		}
		if !enabled {
			log.Printf("[AI分析] 自动评分开关关闭，跳过项目: %s", id)
			return
		}

		log.Printf("[AI分析] 开始异步分析项目: %s", id)
		record, err := fetchBidRecordByID(id)
		if err != nil {
			log.Printf("[AI分析] 获取项目 %s 记录失败: %v", id, err)
			return
		}

		result, err := performBidAnalysis(record, "", "auto", "")
		if err != nil {
			log.Printf("[AI分析] 项目 %s 分析失败: %v", id, err)
		} else {
			log.Printf("[AI分析] 项目 %s 分析完成，模型: %s", id, result.AIModel)

			// 发送微信通知
			go sendWechatAfterSingleAnalysis(record, result)
		}
	}(id)

}
