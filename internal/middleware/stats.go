package middleware

import (
	"log"
	"time" // 导入 time 包

	"go-proxy/internal/db"
	"go-proxy/pkg/config"
	"go-proxy/pkg/types" // 导入新的 types 包

	"github.com/labstack/echo/v4"
)

const (
	// DefaultBatchSize 定义批量处理中项目的默认数量
	DefaultBatchSize = 50
	// DefaultBatchTimeout 定义批量处理的默认超时时间（秒）
	DefaultBatchTimeout = 1 * time.Second
)

// StatsChannel 全局的 channel 用于处理统计
var StatsChannel chan types.RequestStat // 使用 types.RequestStat

// InitStatsChannel 初始化统计通道
func InitStatsChannel(bufferSize int) {
	StatsChannel = make(chan types.RequestStat, bufferSize) // 使用 types.RequestStat
}

// ProcessStats 异步处理统计的函数
func ProcessStats() {
	batchStats := make([]types.RequestStat, 0, DefaultBatchSize)
	batchCounts := make(map[string]int)
	ticker := time.NewTicker(DefaultBatchTimeout)
	defer ticker.Stop()

	processBatch := func() {
		if len(batchStats) > 0 {
			if err := db.BatchLogRequestDetails(batchStats); err != nil {
				log.Printf("批量记录请求详情时出错: %v", err)
				// 根据策略，这里可能需要决定是否重试或如何处理失败的批次
			}
			batchStats = make([]types.RequestStat, 0, DefaultBatchSize) // 重置切片
		}
		if len(batchCounts) > 0 {
			if err := db.BatchIncrementRequestCounts(batchCounts); err != nil {
				log.Printf("批量更新请求计数时出错: %v", err)
				// 根据策略，这里可能需要决定是否重试或如何处理失败的批次
			}
			batchCounts = make(map[string]int) // 重置map
		}
	}

	for {
		select {
		case stat, ok := <-StatsChannel:
			if !ok { // Channel 已关闭
				log.Println("StatsChannel 已关闭，处理剩余批次...")
				processBatch() // 处理关闭前可能剩余的任何数据
				log.Println("统计处理 goroutine 已退出")
				return
			}

			batchStats = append(batchStats, stat)
			batchCounts[stat.ServiceName]++ // 简单地为每个服务名增加计数

			if len(batchStats) >= DefaultBatchSize {
				// log.Printf("批处理大小达到阈值 (%d)，处理批次...", DefaultBatchSize)
				processBatch()
				// 重置定时器，避免刚处理完一批就因为超时又处理空批次
				// (或者让它自然触发，取决于具体需求，如果希望严格按超时，则不需要重置)
				ticker.Reset(DefaultBatchTimeout)
			}

		case <-ticker.C:
			// log.Println("批处理超时，处理批次...")
			processBatch()
		}
	}
}

// StatsMiddleware 创建一个Echo中间件函数，用于增加特定代理路径配置的请求计数。
func StatsMiddleware(proxyCfg config.ProxyConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 增加此特定代理路径的请求计数
			// serviceName := proxyCfg.Path
			// err := db.IncrementRequestCount(serviceName)
			// if err != nil {
			// 	log.Printf("增加路径 %s 的统计信息时出错: %v", serviceName, err)
			// }

			// 调用下一个处理器
			err := next(c)

			// 记录请求的详细信息（包括状态码）
			// host := c.Request().Host
			// requestURI := c.Request().RequestURI
			// statusCode := c.Response().Status
			// logErr := db.LogRequestDetails(serviceName, host, requestURI, statusCode)
			// if logErr != nil {
			// 	log.Printf("记录请求详情时出错: service='%s', host='%s', uri='%s', status='%d', error: %v",
			// 		serviceName, host, requestURI, statusCode, logErr)
			// }
			// 将统计信息发送到 channel
			if StatsChannel != nil {
				// 使用非阻塞发送，如果 channel 满了则记录错误，避免阻塞请求处理
				select {
				case StatsChannel <- types.RequestStat{ // 使用 types.RequestStat
					ServiceName: proxyCfg.Path,
					Host:        c.Request().Host,
					RequestURI:  c.Request().URL.RequestURI(), // 使用 c.Request().URL.RequestURI() 获取原始请求URI
					StatusCode:  c.Response().Status,
				}:
				default:
					log.Println("统计通道已满，部分统计信息可能丢失")
				}
			} else {
				log.Println("StatsChannel is nil, skipping stats logging")
			}

			return err
		}
	}
}
