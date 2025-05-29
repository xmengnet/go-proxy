package middleware

import (
	"log"

	"go-proxy/internal/db"
	"go-proxy/pkg/config"

	"github.com/labstack/echo/v4"
)

// RequestStat 定义一个结构体来传递统计数据
type RequestStat struct {
	ServiceName string
	Host        string
	RequestURI  string
	StatusCode  int
}

// StatsChannel 全局的 channel 用于处理统计
var StatsChannel chan RequestStat

// InitStatsChannel 初始化统计通道
func InitStatsChannel(bufferSize int) {
	StatsChannel = make(chan RequestStat, bufferSize)
}

// ProcessStats 异步处理统计的函数
func ProcessStats() {
	for stat := range StatsChannel {
		err := db.IncrementRequestCount(stat.ServiceName)
		if err != nil {
			log.Printf("异步增加路径 %s 的统计信息时出错: %v", stat.ServiceName, err)
		}
		logErr := db.LogRequestDetails(stat.ServiceName, stat.Host, stat.RequestURI, stat.StatusCode)
		if logErr != nil {
			log.Printf("异步记录请求详情时出错: service='%s', host='%s', uri='%s', status='%d', error: %v",
				stat.ServiceName, stat.Host, stat.RequestURI, stat.StatusCode, logErr)
		}
	}
	log.Println("统计处理 goroutine 已退出")
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
				case StatsChannel <- RequestStat{
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
