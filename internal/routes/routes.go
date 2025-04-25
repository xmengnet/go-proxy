package routes

import (
	"go-proxy/internal/db"
	"go-proxy/internal/middleware"
	"go-proxy/pkg/config"
	"go-proxy/pkg/proxy"
	"net/http"

	"github.com/labstack/echo/v4"
)

func RegisterRoutes(e *echo.Echo, proxies []config.ProxyConfig) {
	// 注册代理路由并应用统计中间件
	for _, p := range proxies {
		// 需要捕获循环变量 p 以供闭包使用
		proxyCfg := p
		reverseProxy := proxy.NewReverseProxy(proxyCfg)
		group := e.Group(proxyCfg.Path)
		// 为这个特定的代理配置应用统计中间件
		group.Use(middleware.StatsMiddleware(proxyCfg))
		group.Any("/*", reverseProxy.Handler)
	}

	// 添加获取统计信息的路由
	e.GET("/api/stats", func(c echo.Context) error {
		// Get statistics from the database
		stats, err := db.GetStats()
		if err != nil {
			c.Logger().Errorf("获取统计信息时出错: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to retrieve statistics"})
		}

		// Create a map for quick lookup of stats by service name
		statsMap := make(map[string]int)
		for _, stat := range stats {
			statsMap[stat.ServiceName] = stat.RequestCount
		}

		// Combine proxy configurations with statistics
		var proxyStats []ProxyStat
		for _, p := range proxies {
			accessCount := 0
			if count, ok := statsMap[p.Path]; ok {
				accessCount = count
			}
			proxyStats = append(proxyStats, ProxyStat{
				ProxyURL:    p.Path,
				SourceURL:   p.Target,
				AccessCount: accessCount,
			})
		}

		return c.JSON(http.StatusOK, proxyStats)
	})

	// Serve static files from the "web" directory
	e.Static("/", "web")
}

// ProxyStat represents the combined proxy configuration and statistics
type ProxyStat struct {
	ProxyURL    string `json:"proxy_url"`
	SourceURL   string `json:"source_url"`
	AccessCount int    `json:"access_count"`
}
