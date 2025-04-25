package middleware

import (
	"log"

	"go-proxy/internal/db"
	"go-proxy/pkg/config"

	"github.com/labstack/echo/v4"
)

// StatsMiddleware 创建一个Echo中间件函数，用于增加特定代理路径配置的请求计数。
func StatsMiddleware(proxyCfg config.ProxyConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 增加此特定代理路径的请求计数
			// 我们使用proxyCfg.Path来唯一标识代理路由
			serviceName := proxyCfg.Path
			err := db.IncrementRequestCount(serviceName)
			if err != nil {
				// 记录错误但不阻止请求
				log.Printf("增加路径 %s 的统计信息时出错: %v", serviceName, err)
			}

			// 记录请求的详细信息
			host := c.Request().Host
			requestURI := c.Request().RequestURI
			err = db.LogRequestDetails(serviceName, host, requestURI)
			if err != nil {
				// 记录错误但不阻止请求
				log.Printf("记录请求详情时出错: service='%s', host='%s', uri='%s', error: %v", serviceName, host, requestURI, err)
			}

			// 继续处理链中的下一个处理器
			return next(c)
		}
	}
}
