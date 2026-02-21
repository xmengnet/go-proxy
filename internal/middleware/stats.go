package middleware

import (
	"go-proxy/internal/db"
	"go-proxy/pkg/config"
	"go-proxy/pkg/types"
	"log"
	"net/http"
	"strings"
	"time"

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

// ProcessStats 异步处理统计的函数。retentionDays 指定数据保留天数。
func ProcessStats(retentionDays int) {
	batchStats := make([]types.RequestStat, 0, DefaultBatchSize)
	batchCounts := make(map[string]int)
	ticker := time.NewTicker(DefaultBatchTimeout)
	defer ticker.Stop()

	processBatch := func() {
		if len(batchStats) == 0 && len(batchCounts) == 0 {
			return // 如果没有数据要处理，直接返回
		}

		// 最多重试3次
		maxRetries := 3
		var lastErr error

		// 处理请求详情
		if len(batchStats) > 0 {
			for attempt := 1; attempt <= maxRetries; attempt++ {
				if err := db.BatchLogRequestDetails(batchStats); err != nil {
					lastErr = err
					log.Printf("批量记录请求详情时出错 (尝试 %d/%d): %v", attempt, maxRetries, err)
					if attempt < maxRetries {
						// 指数退避重试延迟
						time.Sleep(time.Duration(attempt*attempt) * 100 * time.Millisecond)
						continue
					}
				} else {
					lastErr = nil
					break
				}
			}
			if lastErr != nil {
				log.Printf("批量记录请求详情最终失败，丢弃 %d 条记录", len(batchStats))
			}
			batchStats = make([]types.RequestStat, 0, DefaultBatchSize)
		}

		// 处理请求计数
		if len(batchCounts) > 0 {
			for attempt := 1; attempt <= maxRetries; attempt++ {
				if err := db.BatchIncrementRequestCounts(batchCounts); err != nil {
					lastErr = err
					log.Printf("批量更新请求计数时出错 (尝试 %d/%d): %v", attempt, maxRetries, err)
					if attempt < maxRetries {
						time.Sleep(time.Duration(attempt*attempt) * 100 * time.Millisecond)
						continue
					}
				} else {
					lastErr = nil
					break
				}
			}
			if lastErr != nil {
				log.Printf("批量更新请求计数最终失败，丢弃 %d 个服务的计数", len(batchCounts))
			}
			batchCounts = make(map[string]int)
		}
	}

	batchTicker := time.NewTicker(5 * time.Minute)
	defer batchTicker.Stop()

	for {
		select {
		case stat, ok := <-StatsChannel:
			if !ok {
				log.Println("StatsChannel 已关闭，正在处理剩余批次...")
				processBatch()
				log.Println("统计处理 goroutine 已正常退出")
				return
			}

			batchStats = append(batchStats, stat)
			batchCounts[stat.ServiceName]++

			if len(batchStats) >= DefaultBatchSize {
				processBatch()
				ticker.Reset(DefaultBatchTimeout)
			}

		case <-ticker.C:
			processBatch()

		case <-batchTicker.C:
			// 每5分钟：记录状态、聚合每日数据、清理过期数据
			log.Printf("统计处理状态：当前批次大小=%d, 计数映射大小=%d",
				len(batchStats), len(batchCounts))

			// 将 request_logs 聚合到 daily_summary
			if err := db.AggregateDaily(1); err != nil {
				log.Printf("定时聚合每日统计失败: %v", err)
			} else {
				// 聚合成功后清除缓存，让下次 API 查询获取最新数据
				db.InvalidateCache()
			}

			// 清理超过保留期的历史数据
			if err := db.CleanupOldData(retentionDays); err != nil {
				log.Printf("定时清理历史数据失败: %v", err)
			}
		}
	}
}

// StatsMiddleware 创建一个Echo中间件函数，用于增加特定代理路径配置的请求计数。
func StatsMiddleware(proxyCfg config.ProxyConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path

			// 如果请求路径不是以代理配置的路径开头，直接跳过统计
			if !strings.HasPrefix(path, proxyCfg.Path) {
				return next(c)
			}

			// 过滤掉静态文件和前端页面的请求
			if strings.HasSuffix(path, ".html") ||
				strings.HasSuffix(path, ".js") ||
				strings.HasSuffix(path, ".css") ||
				strings.HasSuffix(path, ".svg") ||
				strings.HasSuffix(path, ".ico") ||
				strings.HasSuffix(path, ".png") ||
				strings.HasSuffix(path, ".jpg") ||
				strings.HasPrefix(path, "/api/stats") {
				return next(c)
			}

			start := time.Now()
			var err error

			// 使用 defer 来确保即使发生 panic 也能记录响应时间
			defer func() {
				if r := recover(); r != nil {
					// 记录 panic 并重新触发
					log.Printf("在处理请求时发生 panic: %v", r)
					panic(r)
				}

				duration := time.Since(start)
				responseTime := duration.Milliseconds()

				// 获取状态码，如果出现错误则使用500
				statusCode := c.Response().Status
				if err != nil {
					if he, ok := err.(*echo.HTTPError); ok {
						statusCode = he.Code
					} else {
						statusCode = http.StatusInternalServerError
					}
				}

				// 只记录实际的API调用
				// 检查请求是否应该被统计（可以根据需要添加更多条件）
				shouldCount := statusCode != http.StatusNotFound && // 不统计404
					c.Request().Method != http.MethodOptions && // 不统计OPTIONS请求
					!strings.Contains(c.Request().Header.Get("User-Agent"), "HealthCheck") // 不统计健康检查

				if shouldCount {
					if StatsChannel != nil {
						stat := types.RequestStat{
							ServiceName:  proxyCfg.Path,
							Host:         c.Request().Host,
							RequestURI:   c.Request().URL.RequestURI(),
							StatusCode:   statusCode,
							ResponseTime: responseTime,
						}

						// 使用非阻塞发送
						select {
						case StatsChannel <- stat:
							// 成功发送到通道
						default:
							log.Printf("警告: 统计通道已满 (service=%s, status=%d, time=%dms)",
								proxyCfg.Path, statusCode, responseTime)
						}
					} else {
						log.Println("警告: StatsChannel 未初始化，跳过统计记录")
					}
				}
			}()

			// 调用下一个处理器
			err = next(c)
			return err
		}
	}
}
