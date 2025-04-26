package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

var (
	db   *sql.DB
	once sync.Once
	mu   sync.Mutex // To protect concurrent writes
)

// Stat 表示单个服务的统计信息。
type Stat struct {
	ServiceName  string `json:"service_name"`
	RequestCount int    `json:"request_count"`
}

// InitDB 初始化SQLite数据库连接并创建必要的表。
func InitDB(dataSourceName string) error {
	var err error
	once.Do(func() {
		// 判断目录是否存在
		if _, err := os.Stat("data"); os.IsNotExist(err) {
			if err := os.Mkdir("data", 0755); err != nil {
				log.Fatalf("创建目录时出错: %v", err)
			}
		}
		db, err = sql.Open("sqlite3", dataSourceName)
		if err != nil {
			log.Printf("打开数据库时出错: %v", err)
			return // 如果出错则提前返回
		}

		// 检查连接是否实际可用
		if err = db.Ping(); err != nil {
			log.Printf("ping数据库时出错: %v", err)
			db.Close() // 如果ping失败则关闭连接
			db = nil   // 将db重置为nil
			return
		}

		// 如果表不存在则创建表
		createTableSQL := `
		CREATE TABLE IF NOT EXISTS request_stats (
			service_name TEXT PRIMARY KEY,
			request_count INTEGER NOT NULL DEFAULT 0
		);`
		_, err = db.Exec(createTableSQL)
		if err != nil {
			log.Printf("创建 request_stats 表时出错: %v", err)
			db.Close() // 如果表创建失败则关闭连接
			db = nil   // 将db重置为nil
			return
		}

		// 如果 request_logs 表不存在则创建表
		createLogsTableSQL := `
		CREATE TABLE IF NOT EXISTS request_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			service_name TEXT NOT NULL,
			host TEXT,
			request_uri TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);`
		_, err = db.Exec(createLogsTableSQL)
		if err != nil {
			log.Printf("创建 request_logs 表时出错: %v", err)
			db.Close() // 如果表创建失败则关闭连接
			db = nil   // 将db重置为nil
			return
		}

		log.Println("数据库初始化成功。")
	})
	// 返回once.Do内部捕获的错误或nil（如果成功）
	// 同时检查db是否为nil（表示once.Do内部初始化失败）
	if db == nil && err == nil {
		// 这种情况可能发生在once.Do完成但内部发生错误
		// 且没有正确传播或处理。我们确保返回错误状态。
		return fmt.Errorf("数据库初始化失败")
	}
	return err
}

// CloseDB 关闭数据库连接。
func CloseDB() {
	if db != nil {
		db.Close()
		log.Println("数据库连接已关闭。")
	}
}

// IncrementRequestCount 增加指定服务的请求计数。
// 它使用INSERT ON CONFLICT来处理新服务和现有服务的原子更新。
func IncrementRequestCount(serviceName string) error {
	if db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	mu.Lock()
	defer mu.Unlock()

	// 使用INSERT ... ON CONFLICT ... DO UPDATE进行原子upsert
	query := `
	INSERT INTO request_stats (service_name, request_count) VALUES (?, 1)
	ON CONFLICT(service_name) DO UPDATE SET request_count = request_count + 1;
	`
	_, err := db.Exec(query, serviceName)
	if err != nil {
		log.Printf("增加服务 '%s' 的请求计数时出错: %v", serviceName, err)
	}
	return err
}

// GetStats 从数据库中检索所有请求统计信息。
func GetStats() ([]Stat, error) {
	if db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	mu.Lock() // 锁定以保证读取一致性，尽管读取的临界性不如写入重要
	defer mu.Unlock()

	rows, err := db.Query("SELECT service_name, request_count FROM request_stats ORDER BY request_count DESC")
	if err != nil {
		log.Printf("查询统计信息时出错: %v", err)
		return nil, err
	}
	defer rows.Close()

	stats := []Stat{}
	for rows.Next() {
		var s Stat
		if err := rows.Scan(&s.ServiceName, &s.RequestCount); err != nil {
			log.Printf("扫描统计信息行时出错: %v", err)
			continue // 跳过有问题的行
		}
		stats = append(stats, s)
	}

	if err = rows.Err(); err != nil {
		log.Printf("迭代统计信息行时出错: %v", err)
		return nil, err // 返回行迭代的错误
	}

	return stats, nil
}

// 辅助函数，用于检查数据库是否已初始化（可选，用于内部使用或测试）
func IsInitialized() bool {
	return db != nil
}

// LogRequestDetails 记录单个请求的详细信息。
func LogRequestDetails(serviceName, host, requestURI string) error {
	if db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	// No need for mutex here as INSERTs are typically safe concurrently
	query := `
	INSERT INTO request_logs (service_name, host, request_uri) VALUES (?, ?, ?);
	`
	_, err := db.Exec(query, serviceName, host, requestURI)
	if err != nil {
		log.Printf("记录请求详情时出错: service='%s', host='%s', uri='%s', error: %v", serviceName, host, requestURI, err)
	}
	return err
}
