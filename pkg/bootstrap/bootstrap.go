package bootstrap

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"io/fs"

	"go-proxy/internal/db"
	"go-proxy/internal/middleware"
	"go-proxy/internal/routes"
	"go-proxy/pkg/config"

	"github.com/labstack/echo/v4"
)

const (
	dbPath                 = "data/stats.db"
	defaultPort            = "8080"
	statsChannelBufferSize = 1000
)

// SetupApp 配置并返回一个 Echo 实例。
// isVercelEnv: 指示是否在 Vercel 环境中运行。
// wg: 用于等待后台 goroutine（如 ProcessStats）完成。如果为 nil 且需要后台任务，则会内部创建。
// staticFS: 可选的静态文件系统（通常为 go:embed 生成），nil 时回退到磁盘目录。
func SetupApp(isVercelEnv bool, wg *sync.WaitGroup, staticFS fs.FS) (*echo.Echo, *config.Config, error) {
	e := echo.New()
	var cfg *config.Config
	var err error
	var proxies []config.ProxyConfig

	if isVercelEnv {
		log.Println("在 Vercel 环境中设置应用...")
		// Vercel 环境: 从环境变量加载代理配置
		proxiesJSON := os.Getenv("PROXIES_CONFIG")
		if proxiesJSON != "" {
			if err = json.Unmarshal([]byte(proxiesJSON), &proxies); err != nil {
				log.Printf("无法解析 Vercel PROXIES_CONFIG: %v", err)
				// 在 Vercel 中，配置错误可能是致命的，但这里我们允许应用继续，可能没有代理
				proxies = []config.ProxyConfig{} // 重置为空，避免使用部分解析的配置
			}
		} else {
			log.Println("Vercel PROXIES_CONFIG 未设置，无代理注册。")
			proxies = []config.ProxyConfig{}
		}
		// 对于 Vercel，我们可能没有完整的 Config 对象，或者可以创建一个简化的
		cfg = &config.Config{
			Server:  config.ServerConfig{Port: defaultPort}, // Vercel 通常自己处理端口
			Proxies: proxies,
		}

		// 在 Vercel 环境中跳过本地数据库初始化
		if initErr := db.InitDB(dbPath, true); initErr != nil {
			// 即使 InitDB(true) 应该返回 nil，也处理以防万一
			log.Printf("Vercel 环境下 db.InitDB 意外出错: %v", initErr)
			// 不认为是致命错误，因为 Vercel 环境不应依赖本地 DB
		}
		// 在 Vercel 环境中不启动统计通道和处理器
		log.Println("Vercel 环境：跳过统计通道和后台处理器初始化。")

	} else {
		log.Println("在非 Vercel (本地/服务器) 环境中设置应用...")
		// 非 Vercel 环境: 从文件加载配置
		cfg, err = config.LoadConfig("data/config.yaml")
		if err != nil {
			return nil, nil, fmt.Errorf("加载配置文件失败: %w", err)
		}
		proxies = cfg.Proxies

		// 初始化数据库
		if initErr := db.InitDB(dbPath, false); initErr != nil {
			return nil, nil, fmt.Errorf("初始化数据库失败: %w", initErr)
		}

		// 初始化统计通道
		middleware.InitStatsChannel(statsChannelBufferSize)

		// 启动异步处理统计的 goroutine
		if wg == nil {
			// 如果外部没有提供 WaitGroup，我们不在这里管理它，
			// 假设调用者（如 main.go）会处理 ProcessStats 的生命周期。
			// 或者，如果 SetupApp 要负责，它需要返回 WaitGroup 或一种关闭机制。
			// 为了简单起见，这里假设 main.go 会像以前一样处理它。
			// 如果 ProcessStats 需要被 SetupApp 启动和管理，则需要更复杂的逻辑。
			// 对于当前目标，我们让 main.go 继续管理 ProcessStats 的 goroutine。
			// InitStatsChannel 已经完成，main.go 中的 go middleware.ProcessStats() 将使用它。
			log.Println("统计通道已初始化。ProcessStats goroutine 应由调用者启动。")
		} else {
			// 如果提供了 WaitGroup，则由调用者（main.go）负责 Add 和 Done
			log.Println("统计通道已初始化。ProcessStats goroutine 和 WaitGroup 由调用者管理。")
		}
	}

	// 注册路由
	routes.RegisterRoutes(e, proxies, staticFS)

	return e, cfg, nil
}
