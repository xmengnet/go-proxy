package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"

	"go-proxy/pkg/config" // 添加这行导入
	"go-proxy/pkg/types"  // 导入新的 types 包

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

var (
	db   *sql.DB
	once sync.Once
	mu   sync.Mutex // To protect concurrent writes
)

// Stat 表示单个服务的统计信息。
type Stat struct {
	ServiceName  string  `json:"service_name"`
	RequestCount int     `json:"request_count"`
	Vendor       string  `json:"vendor"`
	Target       string  `json:"target"`
	ResponseTime float64 `json:"response_time"` // 平均响应时间（毫秒）
}

// DailyStat 表示某一天的调用次数统计。
type DailyStat struct {
	Date         string `json:"date"`
	RequestCount int    `json:"request_count"`
}

// ServiceDistribution 表示某个服务在时间范围内的调用次数。
type ServiceDistribution struct {
	ServiceName  string `json:"service_name"`
	RequestCount int    `json:"request_count"`
}

// InitDB 初始化SQLite数据库连接并创建必要的表。
// isVercelEnv 参数用于指示是否在 Vercel 环境中运行。
func InitDB(dataSourceName string, isVercelEnv bool) error {
	if isVercelEnv {
		log.Println("在 Vercel 环境中运行，跳过 SQLite 数据库初始化。")
		db = nil // 确保 db 实例为 nil
		return nil
	}

	var err error
	once.Do(func() {
		// 判断目录是否存在
		dataDir := "data"
		if _, statErr := os.Stat(dataDir); os.IsNotExist(statErr) {
			if mkdirErr := os.Mkdir(dataDir, 0755); mkdirErr != nil {
				// 改为记录错误并返回，而不是 Fatalf，以便调用者可以处理
				err = fmt.Errorf("创建目录 '%s' 时出错: %v", dataDir, mkdirErr)
				log.Printf(err.Error())
				return
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

		// 启用 WAL 模式以提高并发性能
		// WAL 模式允许读取者在写入者写入时继续，这可以显著提高并发性。
		// 应该在数据库连接建立后的早期阶段设置。
		_, err = db.Exec("PRAGMA journal_mode=WAL;")
		if err != nil {
			log.Printf("启用 WAL 模式时出错: %v", err)
			// 根据策略，这里可以选择关闭数据库或继续（如果WAL不是强制性的）
			// 为了安全起见，如果无法启用WAL，我们记录错误但继续
		} else {
			log.Println("WAL 模式已成功启用。")
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

		// 为 request_logs 添加索引以加速按服务和时间的查询
		if _, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_request_logs_service_ts ON request_logs(service_name, timestamp DESC);`); err != nil {
			log.Printf("创建 request_logs 索引时出错: %v", err)
			// 索引失败不应中断服务，记录即可
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

		// 检查 request_logs 表是否存在 response_time 列
		var hasResponseTime bool
		row = db.QueryRow(`
		SELECT COUNT(*) 
		FROM pragma_table_info('request_logs') 
		WHERE name='response_time'
		`)
		if err := row.Scan(&hasResponseTime); err != nil {
			log.Printf("检查 response_time 列时出错: %v", err)
			return
		}

		// 如果 response_time 列不存在，添加它
		if !hasResponseTime {
			_, err = db.Exec(`ALTER TABLE request_logs ADD COLUMN response_time INTEGER DEFAULT 0;`)
			if err != nil {
				log.Printf("添加 response_time 列时出错: %v", err)
				return
			}
			log.Println("成功添加 response_time 列到 request_logs 表")
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

	// 计算最近100条请求的平均响应时间
	query := `
	SELECT
		pc.path AS service_name,
		COALESCE(rs.request_count, 0) AS request_count,
		pc.vendor,
		pc.target,
		COALESCE(ROUND((
			SELECT AVG(response_time) FROM (
				SELECT response_time
				FROM request_logs
				WHERE service_name = pc.path
				AND status_code BETWEEN 200 AND 299
				AND response_time > 0
				AND response_time < 60000
				ORDER BY timestamp DESC
				LIMIT 100
			)
		), 2), 0) AS response_time
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
		if err := rows.Scan(
			&s.ServiceName,
			&s.RequestCount,
			&s.Vendor,
			&s.Target,
			&s.ResponseTime,
		); err != nil {
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

// GetDailyStatsLast7Days 返回最近7天（含今天）的每日请求次数。
func GetDailyStatsLast7Days() ([]DailyStat, error) {
	if db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	query := `
	WITH RECURSIVE dates(day) AS (
	    SELECT date('now','localtime','-6 days')
	    UNION ALL
	    SELECT date(day,'+1 day') FROM dates WHERE day < date('now','localtime')
	)
	SELECT d.day AS date, COALESCE(cnt.total, 0) AS request_count
	FROM dates d
	LEFT JOIN (
	    SELECT date(timestamp,'localtime') AS day, COUNT(*) AS total
	    FROM request_logs
	    WHERE timestamp >= datetime('now','localtime','-6 days')
	    GROUP BY day
	) cnt ON d.day = cnt.day
	ORDER BY d.day;
	`

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("查询每日统计信息时出错: %v", err)
		return nil, err
	}
	defer rows.Close()

	stats := []DailyStat{}
	for rows.Next() {
		var s DailyStat
		if err := rows.Scan(&s.Date, &s.RequestCount); err != nil {
			log.Printf("扫描每日统计信息行时出错: %v", err)
			continue
		}
		stats = append(stats, s)
	}

	if err = rows.Err(); err != nil {
		log.Printf("迭代每日统计信息行时出错: %v", err)
		return nil, err
	}

	return stats, nil
}

// GetServiceDistributionLast7Days 返回最近7天内各服务的调用次数分布。
func GetServiceDistributionLast7Days() ([]ServiceDistribution, error) {
	if db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	query := `
	SELECT service_name, COUNT(*) AS request_count
	FROM request_logs
	WHERE timestamp >= datetime('now','localtime','-6 days')
	GROUP BY service_name
	ORDER BY request_count DESC;
	`

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("查询服务调用分布时出错: %v", err)
		return nil, err
	}
	defer rows.Close()

	stats := []ServiceDistribution{}
	for rows.Next() {
		var s ServiceDistribution
		if err := rows.Scan(&s.ServiceName, &s.RequestCount); err != nil {
			log.Printf("扫描服务调用分布行时出错: %v", err)
			continue
		}
		stats = append(stats, s)
	}

	if err = rows.Err(); err != nil {
		log.Printf("迭代服务调用分布行时出错: %v", err)
		return nil, err
	}

	return stats, nil
}

// 辅助函数，用于检查数据库是否已初始化（可选，用于内部使用或测试）
func IsInitialized() bool {
	return db != nil
}

// BatchLogRequestDetails 批量记录请求的详细信息。
func BatchLogRequestDetails(stats []types.RequestStat) error { // 使用 types.RequestStat
	if db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if len(stats) == 0 {
		return nil
	}

	// mu.Lock() // 从这里移除 mu.Lock()
	// defer mu.Unlock() // 从这里移除 mu.Unlock()

	tx, err := db.Begin()
	if err != nil {
		log.Printf("开始事务时出错 (BatchLogRequestDetails): %v", err)
		return err
	}
	defer tx.Rollback() // 确保在出错时回滚

	// 为提高性能和避免SQL注入，使用预编译语句
	stmt, err := tx.Prepare(`
		INSERT INTO request_logs 
		(service_name, host, request_uri, status_code, response_time) 
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		log.Printf("准备批量插入 request_logs 语句时出错: %v", err)
		return err
	}
	defer stmt.Close()

	for _, stat := range stats {
		_, err := stmt.Exec(
			stat.ServiceName,
			stat.Host,
			stat.RequestURI,
			stat.StatusCode,
			stat.ResponseTime,
		)
		if err != nil {
			log.Printf("执行批量插入 request_logs (service: %s) 时出错: %v", stat.ServiceName, err)
		}
	}

	return tx.Commit()
}

// BatchIncrementRequestCounts 批量增加指定服务的请求计数。
func BatchIncrementRequestCounts(counts map[string]int) error {
	if db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if len(counts) == 0 {
		return nil
	}

	mu.Lock() // 保护对 request_stats 的并发写操作
	defer mu.Unlock()

	tx, err := db.Begin()
	if err != nil {
		log.Printf("开始事务时出错 (BatchIncrementRequestCounts): %v", err)
		return err
	}
	defer tx.Rollback()

	query := `
	INSERT INTO request_stats (service_name, request_count) VALUES (?, ?)
	ON CONFLICT(service_name) DO UPDATE SET request_count = request_stats.request_count + excluded.request_count;
	`
	stmt, err := tx.Prepare(query)
	if err != nil {
		log.Printf("准备批量更新 request_stats 语句时出错: %v", err)
		return err
	}
	defer stmt.Close()

	for serviceName, incrementValue := range counts {
		if _, err := stmt.Exec(serviceName, incrementValue); err != nil {
			log.Printf("批量更新服务 '%s' 的请求计数时出错: %v", serviceName, err)
			// 决定是继续还是返回错误。这里选择继续，记录错误。
		}
	}

	return tx.Commit()
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

// GetAverageResponseTime 获取指定服务最近的平均响应时间（毫秒）
func GetAverageResponseTime(serviceName string, limit int) (float64, error) {
	if db == nil {
		return 0, fmt.Errorf("数据库未初始化")
	}

	query := `
		SELECT AVG(response_time) 
		FROM (
			SELECT response_time 
			FROM request_logs 
			WHERE service_name = ? 
			ORDER BY timestamp DESC 
			LIMIT ?
		)
	`
	var avg sql.NullFloat64
	err := db.QueryRow(query, serviceName, limit).Scan(&avg)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	if !avg.Valid {
		return 0, nil
	}
	return avg.Float64, nil
}

// GetOverallAverageResponseTime 获取所有服务最近的平均响应时间（毫秒）
func GetOverallAverageResponseTime(limit int) (float64, error) {
	if db == nil {
		return 0, fmt.Errorf("数据库未初始化")
	}

	query := `
		SELECT AVG(response_time) 
		FROM (
			SELECT response_time 
			FROM request_logs 
			ORDER BY timestamp DESC 
			LIMIT ?
		)
	`
	var avg sql.NullFloat64
	err := db.QueryRow(query, limit).Scan(&avg)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	if !avg.Valid {
		return 0, nil
	}
	return avg.Float64, nil
}
