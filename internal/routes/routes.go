package routes

import (
	"go-proxy/internal/db"
	"go-proxy/internal/middleware"
	"go-proxy/pkg/config"
	"go-proxy/pkg/proxy"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

func RegisterRoutes(e *echo.Echo, proxies []config.ProxyConfig) {
	// 检查数据库和统计功能是否应该被启用
	// db.IsInitialized() 检查数据库是否已成功初始化
	// middleware.StatsChannel != nil 检查统计通道是否已初始化
	enableStatsFeatures := db.IsInitialized() && middleware.StatsChannel != nil

	if enableStatsFeatures {
		// 更新代理配置到数据库
		if err := db.UpdateProxyConfig(proxies); err != nil {
			log.Printf("更新代理配置时出错: %v", err)
		}
	} else {
		log.Println("数据库或统计通道未初始化，跳过数据库中的代理配置更新。")
	}

	// 注册代理路由
	for _, p := range proxies {
		// 需要捕获循环变量 p 以供闭包使用
		proxyCfg := p
		reverseProxy := proxy.NewReverseProxy(proxyCfg)
		group := e.Group(proxyCfg.Path)

		if enableStatsFeatures {
			// 为这个特定的代理配置应用统计中间件
			group.Use(middleware.StatsMiddleware(proxyCfg))
		}
		group.Any("/*", reverseProxy.Handler)
	}

	if enableStatsFeatures {
		// 修改获取统计信息的路由
		e.GET("/api/stats", func(c echo.Context) error {
			stats, err := db.GetStats()
			if err != nil {
				c.Logger().Errorf("获取统计信息时出错: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to retrieve statistics"})
			}
			return c.JSON(http.StatusOK, stats)
		})
		log.Println("统计 API (/api/stats) 和中间件已启用。")
	} else {
		log.Println("统计 API (/api/stats) 和中间件已禁用。")
	}

	// Serve static files from the "public" directory
	// 静态文件服务应该总是启用
	e.Static("/", "public")
}

// ProxyStat represents the combined proxy configuration and statistics
type ProxyStat struct {
	ProxyURL    string `json:"proxy_url"`
	SourceURL   string `json:"source_url"`
	AccessCount int    `json:"access_count"`
}
