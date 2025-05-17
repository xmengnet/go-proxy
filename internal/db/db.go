package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"

	"go-proxy/pkg/config" // 添加这行导入

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
	Vendor       string `json:"vendor"` // 添加厂商字段
	Target       string `json:"target"` // 添加目标地址字段
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
			status_code INTEGER,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);`
		_, err = db.Exec(createLogsTableSQL)
		if err != nil {
			log.Printf("创建 request_logs 表时出错: %v", err)
			db.Close() // 如果表创建失败则关闭连接
			db = nil   // 将db重置为nil
			return
		}

		// 检查 request_logs 表是否存在 status_code 列
		var hasStatusCode bool
		row := db.QueryRow(`
		SELECT COUNT(*) 
		FROM pragma_table_info('request_logs') 
		WHERE name='status_code'
		`)
		if err := row.Scan(&hasStatusCode); err != nil {
			log.Printf("检查 status_code 列时出错: %v", err)
			return
		}

		// 如果 status_code 列不存在，添加它
		if !hasStatusCode {
			_, err = db.Exec(`ALTER TABLE request_logs ADD COLUMN status_code INTEGER DEFAULT 0;`)
			if err != nil {
				log.Printf("添加 status_code 列时出错: %v", err)
				return
			}
			log.Println("成功添加 status_code 列到 request_logs 表")
		}

		// 创建代理配置表
		createProxyConfigTableSQL := `
		CREATE TABLE IF NOT EXISTS proxy_config (
		path TEXT PRIMARY KEY,
		target TEXT NOT NULL,
		vendor TEXT
		);`
		_, err = db.Exec(createProxyConfigTableSQL)
		if err != nil {
			log.Printf("创建 proxy_config 表时出错: %v", err)
			db.Close()
			db = nil
			return
		}

		log.Println("数据库初始化成功。")
	})
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
	mu.Lock()
	defer mu.Unlock()

	// 修改 SQL 查询以从 proxy_config 表开始
	query := `
	SELECT 
	    pc.path as service_name,
	    COALESCE(rs.request_count, 0) as request_count,
	    pc.vendor,
	    pc.target
	FROM proxy_config pc
	LEFT JOIN request_stats rs ON pc.path = rs.service_name
	ORDER BY COALESCE(rs.request_count, 0) DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("查询统计信息时出错: %v", err)
		return nil, err
	}
	defer rows.Close()

	stats := []Stat{}
	for rows.Next() {
		var s Stat
		if err := rows.Scan(&s.ServiceName, &s.RequestCount, &s.Vendor, &s.Target); err != nil {
			log.Printf("扫描统计信息行时出错: %v", err)
			continue
		}
		stats = append(stats, s)
	}

	if err = rows.Err(); err != nil {
		log.Printf("迭代统计信息行时出错: %v", err)
		return nil, err
	}

	return stats, nil
}

// 辅助函数，用于检查数据库是否已初始化（可选，用于内部使用或测试）
func IsInitialized() bool {
	return db != nil
}

// LogRequestDetails 记录单个请求的详细信息。
func LogRequestDetails(serviceName, host, requestURI string, statusCode int) error {
	if db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	query := `
    INSERT INTO request_logs (service_name, host, request_uri, status_code) VALUES (?, ?, ?, ?);
    `
	_, err := db.Exec(query, serviceName, host, requestURI, statusCode)
	if err != nil {
		log.Printf("记录请求详情时出错: service='%s', host='%s', uri='%s', status='%d', error: %v",
			serviceName, host, requestURI, statusCode, err)
	}
	return err
}

// 添加更新代理配置的函数
func UpdateProxyConfig(configs []config.ProxyConfig) error {
	if db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	mu.Lock()
	defer mu.Unlock()

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 清空现有配置
	_, err = tx.Exec("DELETE FROM proxy_config")
	if err != nil {
		return err
	}

	// 插入新配置
	stmt, err := tx.Prepare("INSERT INTO proxy_config (path, target, vendor) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, cfg := range configs {
		_, err = stmt.Exec(cfg.Path, cfg.Target, cfg.Vendor)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
