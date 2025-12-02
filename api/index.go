package api

import (
	"log"
	"net/http"

	// "os" // os.Getenv is now in bootstrap
	// "encoding/json" // json.Unmarshal is now in bootstrap

	"go-proxy/pkg/bootstrap" // 更新导入路径
	// "go-proxy/pkg/config" // config types used by bootstrap
	// "go-proxy/pkg/proxy" // proxy logic used by routes registered in bootstrap

	"github.com/labstack/echo/v4"
)

var e *echo.Echo

func init() {
	// true 表示 Vercel 环境, nil for WaitGroup as Vercel doesn't run background tasks like ProcessStats
	appInstance, _, err := bootstrap.SetupApp(true, nil, nil)
	if err != nil {
		// 在 Vercel 的 init 中，错误通常会导致部署失败或函数无法启动
		// 使用 log.Fatalf 或 panic 来确保错误被 Vercel 捕获
		log.Fatalf("Vercel 应用设置失败: %v", err)
	}
	e = appInstance

	// SetupApp(true, nil) 内部已经处理了：
	// - 从 os.Getenv("PROXIES_CONFIG") 加载代理配置
	// - 初始化 Echo 实例 (e)
	// - 根据 Vercel 环境跳过数据库和统计功能初始化
	// - 注册路由 (包括代理路由，但不包括 /api/stats 和 StatsMiddleware)

	// 可选：为Vercel函数URL本身添加根路径处理器。
	// 如果 bootstrap.SetupApp 没有处理这个，可以在这里添加。
	// 假设 bootstrap.SetupApp 中的 routes.RegisterRoutes 已经处理了所有必要的路由。
	// 如果需要一个特定的根路径处理器仅用于 Vercel，可以如下添加：
	// e.GET("/", func(c echo.Context) error {
	// 	return c.String(http.StatusOK, "Go Proxy Serverless Function (via api/index.go) is running!")
	// })
}

// Handler is the main entry point for Vercel Serverless Functions.
// Vercel expects an http.Handler function.
func Handler(w http.ResponseWriter, r *http.Request) {
	if e == nil {
		// 这是一个故障安全措施，理论上 init() 应该已经设置了 e
		// 或者在 init() 失败时已经 panic/fatalf 了。
		log.Println("错误: Echo 实例在 Handler 中为 nil。可能是 init() 失败。")
		http.Error(w, "内部服务器错误: 应用未正确初始化", http.StatusInternalServerError)
		return
	}
	// Serve the request using the initialized Echo instance
	e.ServeHTTP(w, r)
}
