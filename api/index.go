package api

import (
	"encoding/json"
	"net/http"
	"os"

	"go-proxy/pkg/config"
	"go-proxy/pkg/proxy"

	"github.com/labstack/echo/v4"
)

// ProxyStat represents the combined proxy configuration and statistics for the frontend
type ProxyStat struct {
	ProxyURL    string `json:"proxy_url"`
	SourceURL   string `json:"source_url"`
	AccessCount int    `json:"access_count"`
}

var e *echo.Echo

func init() {
	// 初始化Echo框架实例，用于处理HTTP请求和响应。
	e = echo.New()

	// 从环境变量中加载代理配置。
	// 环境变量PROXIES_CONFIG应包含一个JSON数组，其中每个元素是一个代理配置。
	// 示例：'[{"path":"/api1","target":"http://service1.example.com"},{"path":"/api2","target":"http://service2.example.com"}]'
	proxiesJSON := os.Getenv("PROXIES_CONFIG")
	var proxies []config.ProxyConfig // 使用config包中的ProxyConfig结构体来存储解析后的代理配置
	if proxiesJSON != "" {           // 如果环境变量不为空
		err := json.Unmarshal([]byte(proxiesJSON), &proxies) // 将JSON字符串解析为ProxyConfig切片
		if err != nil {                                      // 如果解析失败
			// 记录致命错误日志，并终止程序运行。因为配置是关键部分，解析失败会导致服务无法正常启动。
			// 在无服务器环境下，初始化阶段的致命错误会阻止函数启动。
			e.Logger.Fatalf("无法解析PROXIES_CONFIG环境变量: %v", err)
		}
	} else { // 如果环境变量为空
		// 记录警告日志，提示没有找到代理配置，因此不会注册任何代理。
		e.Logger.Warn("PROXIES_CONFIG环境变量未设置。不会有代理被注册。")
	}

	// 注册代理路由
	for _, p := range proxies { // 遍历所有解析出的代理配置
		// 捕获循环变量p，以确保在闭包中使用的是当前的配置。
		proxyCfg := p
		reverseProxy := proxy.NewReverseProxy(proxyCfg) // 创建一个新的反向代理实例
		group := e.Group(proxyCfg.Path)                 // 为代理路径创建一个路由组
		// 注册匹配该路径的所有请求到反向代理处理器。
		// 注意：为了适配无服务器环境，移除了统计中间件和数据库相关功能。
		group.Any("/*", reverseProxy.Handler) // 匹配所有方法的请求，并交由反向代理处理
	}

	// 从环境变量的配置文件重新实现 /api/stats 接口
	e.GET("/api/stats", func(c echo.Context) error {
		proxiesJSON := os.Getenv("PROXIES_CONFIG")
		var proxies []config.ProxyConfig
		if proxiesJSON != "" {
			err := json.Unmarshal([]byte(proxiesJSON), &proxies)
			if err != nil {
				// Log error but don't fail the request
				e.Logger.Errorf("无法解析PROXIES_CONFIG环境变量: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "无法获取代理配置"})
			}
		} else {
			// 如果没有设置环境变量返回空列表
			proxies = []config.ProxyConfig{}
		}

		// 将 config.ProxyConfig 映射到前端期望的 ProxyStat 结构体
		// 在 Vercel 环境中无法获取请求计数，因此设置为 0
		var proxyStats []ProxyStat
		for _, p := range proxies {
			proxyStats = append(proxyStats, ProxyStat{
				ProxyURL:    p.Path,
				SourceURL:   p.Target,
				AccessCount: 0, // 在 Vercel 环境中无法获取请求计数
			})
		}

		return c.JSON(http.StatusOK, proxyStats)
	})

}

// Handler is the main entry point for Vercel Serverless Functions.
// Vercel expects an http.Handler function.
func Handler(w http.ResponseWriter, r *http.Request) {
	// Serve the request using the initialized Echo instance
	e.ServeHTTP(w, r)
}
