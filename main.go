package main

import (
	"context"
	"go-proxy/internal/config"
	"go-proxy/internal/db" // Import db package
	"go-proxy/internal/routes"
	"log"
	"net/http" // Moved import here
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
)

const dbPath = "data/stats.db" // Define database file path

func main() {
	// 初始化数据库
	if err := db.InitDB(dbPath); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	// 确保程序退出时关闭数据库连接
	defer db.CloseDB()

	// 创建一个新的Echo实例
	e := echo.New()
	// 加载配置文件
	cfg, err := config.LoadConfig("data/config.yaml")
	if err != nil {
		// 使用log.Fatalf保持错误处理的一致性
		log.Fatalf("加载配置文件失败: %v", err)
	}
	// 确保端口格式正确，添加冒号前缀
	serverPort := ":" + cfg.Server.Port

	// 注册路由
	routes.RegisterRoutes(e, cfg.Proxies)

	// 在goroutine中启动服务器，以避免阻塞主goroutine
	go func() {
		if err := e.Start(serverPort); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatalf("服务器启动失败: %v", err)
		}
	}()

	// 创建一个通道来接收系统中断信号（如Ctrl+C）
	quit := make(chan os.Signal, 1)
	// 通知系统将SIGINT和SIGTERM信号发送到quit通道
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	// 阻塞直到接收到信号
	<-quit
	log.Println("正在关闭服务器...")

	// 创建一个带有10秒超时的上下文，用于优雅地关闭服务器
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 尝试优雅地关闭服务器
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal("服务器强制关闭: ", err)
	}

	log.Println("服务器退出")
}
