package main

import (
	"context"
	"embed"
	"go-proxy/internal/db"         // 仍然需要 db.CloseDB
	"go-proxy/internal/middleware" // 仍然需要 middleware.ProcessStats
	"go-proxy/pkg/bootstrap"       // 更新导入路径
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// const dbPath = "data/stats.db"      // Define database file path - 移至 bootstrap
const defaultPort = "8080" // Define a default port - 移至 bootstrap 或由 bootstrap 内的 cfg 控制
// const statsChannelBufferSize = 1000 // 定义统计通道的缓冲区大小 - 移至 bootstrap

func main() {
	var wg sync.WaitGroup
	// 调用 bootstrap.SetupApp 进行应用设置
	// false 表示非 Vercel 环境
	e, cfg, err := bootstrap.SetupApp(false, &wg, getStaticFS())
	if err != nil {
		log.Fatalf("应用设置失败: %v", err)
	}

	// 确保程序退出时关闭数据库连接 (如果已初始化)
	// db.CloseDB() 会检查 db 实例是否为 nil
	defer db.CloseDB()

	// ProcessStats goroutine 的启动和 WaitGroup 管理保留在 main.go 中
	// SetupApp(false, &wg) 内部已经调用了 middleware.InitStatsChannel
	wg.Add(1)
	go func() {
		defer wg.Done()
		middleware.ProcessStats(cfg.Server.RetentionDays)
		log.Println("ProcessStats goroutine 已退出。")
	}()

	// Determine the server port from config returned by SetupApp
	serverPort := cfg.Server.Port
	if serverPort == "" {
		// bootstrap 中的 defaultPort 应该已经处理了，但作为后备
		serverPort = defaultPort // 使用 bootstrap 中定义的 defaultPort 或 cfg 中的
		log.Printf("配置文件中未指定端口或 bootstrap 未设置，使用后备默认端口: %s", serverPort)
	}
	serverAddr := ":" + serverPort

	// 路由已在 SetupApp 中注册

	// 在goroutine中启动服务器，以避免阻塞主goroutine
	go func() {
		if err := e.Start(serverAddr); err != nil && err != http.ErrServerClosed {
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

//go:embed public/*
var embeddedPublic embed.FS

func getStaticFS() fs.FS {
	sub, err := fs.Sub(embeddedPublic, "public")
	if err != nil {
		log.Fatalf("初始化静态资源失败: %v", err)
	}
	return sub
}
