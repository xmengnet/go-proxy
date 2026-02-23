package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"go-proxy/pkg/config"
	"go-proxy/pkg/types"

	_ "modernc.org/sqlite" // 纯 Go SQLite driver，无需 CGO
)

var (
	db   *sql.DB
	once sync.Once
	mu   sync.Mutex // To protect concurrent writes
)

// ========================================
// 缓存层
// ========================================

type statsCache struct {
	mu            sync.RWMutex
	stats         []Stat
	daily         []DailyStat
	distribution  []ServiceDistribution
	statsExpiry   time.Time
	dailyExpiry   time.Time
	distExpiry    time.Time
	cacheDuration time.Duration
}

var cache = &statsCache{
	cacheDuration: 60 * time.Second,
}

// InvalidateCache 清除所有缓存，在聚合任务完成后调用。
func InvalidateCache() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.statsExpiry = time.Time{}
	cache.dailyExpiry = time.Time{}
	cache.distExpiry = time.Time{}
	log.Println("统计缓存已失效。")
}

// ========================================
// 数据类型
// ========================================

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

// ========================================
// 初始化
// ========================================

// InitDB 初始化SQLite数据库连接并创建必要的表。
func InitDB(dataSourceName string, isVercelEnv bool) error {
	if isVercelEnv {
		log.Println("在 Vercel 环境中运行，跳过 SQLite 数据库初始化。")
		db = nil
		return nil
	}

	var err error
	once.Do(func() {
		dataDir := "data"
		if _, statErr := os.Stat(dataDir); os.IsNotExist(statErr) {
			if mkdirErr := os.Mkdir(dataDir, 0755); mkdirErr != nil {
				err = fmt.Errorf("创建目录 '%s' 时出错: %v", dataDir, mkdirErr)
				log.Printf(err.Error())
				return
			}
		}
		db, err = sql.Open("sqlite", dataSourceName)
		if err != nil {
			log.Printf("打开数据库时出错: %v", err)
			return
		}

		if err = db.Ping(); err != nil {
			log.Printf("ping数据库时出错: %v", err)
			db.Close()
			db = nil
			return
		}

		// 启用 WAL 模式
		_, err = db.Exec("PRAGMA journal_mode=WAL;")
		if err != nil {
			log.Printf("启用 WAL 模式时出错: %v", err)
		} else {
			log.Println("WAL 模式已成功启用。")
		}

		// 建表：request_stats
		_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS request_stats (
			service_name TEXT PRIMARY KEY,
			request_count INTEGER NOT NULL DEFAULT 0
		);`)
		if err != nil {
			log.Printf("创建 request_stats 表时出错: %v", err)
			db.Close()
			db = nil
			return
		}

		// 建表：request_logs
		_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS request_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			service_name TEXT NOT NULL,
			host TEXT,
			request_uri TEXT,
			status_code INTEGER,
			response_time INTEGER DEFAULT 0,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);`)
		if err != nil {
			log.Printf("创建 request_logs 表时出错: %v", err)
			db.Close()
			db = nil
			return
		}

		// 索引：request_logs
		if _, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_request_logs_service_ts ON request_logs(service_name, timestamp DESC);`); err != nil {
			log.Printf("创建 request_logs 索引时出错: %v", err)
		}

		// 兼容迁移：添加可能缺失的列
		addColumnIfNotExists("request_logs", "status_code", "INTEGER DEFAULT 0")
		addColumnIfNotExists("request_logs", "response_time", "INTEGER DEFAULT 0")

		// 建表：proxy_config
		_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS proxy_config (
			path TEXT PRIMARY KEY,
			target TEXT NOT NULL,
			vendor TEXT
		);`)
		if err != nil {
			log.Printf("创建 proxy_config 表时出错: %v", err)
			db.Close()
			db = nil
			return
		}

		// 建表：daily_summary（预聚合表）
		_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS daily_summary (
			date TEXT NOT NULL,
			service_name TEXT NOT NULL,
			request_count INTEGER NOT NULL DEFAULT 0,
			success_count INTEGER NOT NULL DEFAULT 0,
			total_response_time INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (date, service_name)
		);`)
		if err != nil {
			log.Printf("创建 daily_summary 表时出错: %v", err)
			db.Close()
			db = nil
			return
		}

		log.Println("数据库初始化成功。")
	})
	return err
}

// addColumnIfNotExists 检查并添加缺失的列。
func addColumnIfNotExists(table, column, colType string) {
	var count int
	row := db.QueryRow(fmt.Sprintf(
		"SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name='%s'",
		table, column,
	))
	if err := row.Scan(&count); err != nil {
		log.Printf("检查 %s.%s 列时出错: %v", table, column, err)
		return
	}
	if count == 0 {
		_, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;", table, column, colType))
		if err != nil {
			log.Printf("添加 %s.%s 列时出错: %v", table, column, err)
			return
		}
		log.Printf("成功添加 %s 列到 %s 表", column, table)
	}
}

// CloseDB 关闭数据库连接。
func CloseDB() {
	if db != nil {
		db.Close()
		log.Println("数据库连接已关闭。")
	}
}

// IsInitialized 检查数据库是否已初始化。
func IsInitialized() bool {
	return db != nil
}

// ========================================
// 聚合与清理
// ========================================

// IsSummaryEmpty 检查 daily_summary 表是否为空。
func IsSummaryEmpty() bool {
	if db == nil {
		return true
	}
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM daily_summary").Scan(&count)
	if err != nil {
		log.Printf("检查 daily_summary 时出错: %v", err)
		return true
	}
	return count == 0
}

// AggregateDaily 将 request_logs 中指定天数内的数据聚合到 daily_summary 表。
// daysBack: 向前聚合多少天。首次运行传较大值覆盖全部历史，后续传1即可。
func AggregateDaily(daysBack int) error {
	if db == nil {
		return fmt.Errorf("数据库未初始化")
	}

	query := fmt.Sprintf(`
	INSERT OR REPLACE INTO daily_summary (date, service_name, request_count, success_count, total_response_time)
	SELECT 
		date(timestamp, 'localtime') AS day,
		service_name,
		COUNT(*) AS request_count,
		SUM(CASE WHEN status_code BETWEEN 200 AND 299 THEN 1 ELSE 0 END) AS success_count,
		SUM(CASE WHEN response_time > 0 AND response_time < 60000 THEN response_time ELSE 0 END) AS total_response_time
	FROM request_logs
	WHERE date(timestamp, 'localtime') >= date('now', 'localtime', '-%d days')
	GROUP BY day, service_name;
	`, daysBack)

	_, err := db.Exec(query)
	if err != nil {
		log.Printf("聚合每日统计时出错: %v", err)
		return err
	}
	log.Printf("每日统计聚合完成（回溯 %d 天）。", daysBack)
	return nil
}

// CleanupOldData 删除超过指定天数的 request_logs 和 daily_summary 数据。
// doVacuum 为 true 时额外执行 VACUUM 归还磁盘空间（耗时较长，建议每天执行一次）。
func CleanupOldData(retentionDays int, doVacuum bool) error {
	if db == nil {
		return fmt.Errorf("数据库未初始化")
	}

	// 分批删除 request_logs，每次最多删除 10000 条，避免长时间锁表
	deleteLogsQuery := fmt.Sprintf(`
	DELETE FROM request_logs WHERE id IN (
		SELECT id FROM request_logs 
		WHERE timestamp < datetime('now', 'localtime', '-%d days')
		LIMIT 10000
	);`, retentionDays)

	result, err := db.Exec(deleteLogsQuery)
	if err != nil {
		log.Printf("清理旧 request_logs 时出错: %v", err)
		return err
	}
	if rows, _ := result.RowsAffected(); rows > 0 {
		log.Printf("已清理 %d 条过期 request_logs 记录。", rows)
	}

	// 清理 daily_summary
	deleteSummaryQuery := fmt.Sprintf(`
	DELETE FROM daily_summary WHERE date < date('now', 'localtime', '-%d days');
	`, retentionDays)

	result, err = db.Exec(deleteSummaryQuery)
	if err != nil {
		log.Printf("清理旧 daily_summary 时出错: %v", err)
		return err
	}
	if rows, _ := result.RowsAffected(); rows > 0 {
		log.Printf("已清理 %d 条过期 daily_summary 记录。", rows)
	}

	// VACUUM：归还已删除数据占用的磁盘空间。
	// SQLite DELETE 只将页面加入 freelist，不会自动缩减文件，必须显式执行 VACUUM。
	if doVacuum {
		log.Println("开始执行 VACUUM，回收数据库磁盘空间...")
		if _, err = db.Exec("VACUUM;"); err != nil {
			log.Printf("VACUUM 执行失败: %v", err)
		} else {
			log.Println("VACUUM 完成，数据库文件已压缩。")
		}
	}

	return nil
}

// ========================================
// 统计查询（带缓存）
// ========================================

// GetStatsWithCache 带缓存的统计查询。
func GetStatsWithCache() ([]Stat, error) {
	cache.mu.RLock()
	if time.Now().Before(cache.statsExpiry) && cache.stats != nil {
		defer cache.mu.RUnlock()
		return cache.stats, nil
	}
	cache.mu.RUnlock()

	stats, err := GetStats()
	if err != nil {
		return nil, err
	}

	cache.mu.Lock()
	cache.stats = stats
	cache.statsExpiry = time.Now().Add(cache.cacheDuration)
	cache.mu.Unlock()
	return stats, nil
}

// GetDailyStatsWithCache 带缓存的每日统计查询。
func GetDailyStatsWithCache() ([]DailyStat, error) {
	cache.mu.RLock()
	if time.Now().Before(cache.dailyExpiry) && cache.daily != nil {
		defer cache.mu.RUnlock()
		return cache.daily, nil
	}
	cache.mu.RUnlock()

	stats, err := GetDailyStatsLast7Days()
	if err != nil {
		return nil, err
	}

	cache.mu.Lock()
	cache.daily = stats
	cache.dailyExpiry = time.Now().Add(cache.cacheDuration)
	cache.mu.Unlock()
	return stats, nil
}

// GetDistributionWithCache 带缓存的服务分布查询。
func GetDistributionWithCache() ([]ServiceDistribution, error) {
	cache.mu.RLock()
	if time.Now().Before(cache.distExpiry) && cache.distribution != nil {
		defer cache.mu.RUnlock()
		return cache.distribution, nil
	}
	cache.mu.RUnlock()

	stats, err := GetServiceDistributionLast7Days()
	if err != nil {
		return nil, err
	}

	cache.mu.Lock()
	cache.distribution = stats
	cache.distExpiry = time.Now().Add(cache.cacheDuration)
	cache.mu.Unlock()
	return stats, nil
}

// ========================================
// 统计查询（改造后，从 daily_summary 读取）
// ========================================

// GetStats 从数据库中检索所有请求统计信息。
// 平均响应时间改为从 daily_summary 计算最近7天的平均值。
func GetStats() ([]Stat, error) {
	if db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	// 分母使用 request_count（所有请求数），避免因 success_count 极小（大量非2xx响应）
	// 导致平均响应时间虚高（例如出现 3000000ms 这类异常值）。
	query := `
	SELECT
		pc.path AS service_name,
		COALESCE(rs.request_count, 0) AS request_count,
		pc.vendor,
		pc.target,
		COALESCE(ROUND(
			(SELECT CAST(SUM(total_response_time) AS REAL) / NULLIF(SUM(request_count), 0)
			 FROM daily_summary
			 WHERE service_name = pc.path
			 AND date >= date('now','localtime','-7 days')
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
// 改为从 daily_summary 表查询。
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
	SELECT d.day AS date, COALESCE(SUM(ds.request_count), 0) AS request_count
	FROM dates d
	LEFT JOIN daily_summary ds ON d.day = ds.date
	GROUP BY d.day
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
// 改为从 daily_summary 表查询。
func GetServiceDistributionLast7Days() ([]ServiceDistribution, error) {
	if db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	query := `
	SELECT service_name, SUM(request_count) AS request_count
	FROM daily_summary
	WHERE date >= date('now','localtime','-6 days')
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

// ========================================
// 写入操作
// ========================================

// BatchLogRequestDetails 批量记录请求的详细信息。
func BatchLogRequestDetails(stats []types.RequestStat) error {
	if db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if len(stats) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("开始事务时出错 (BatchLogRequestDetails): %v", err)
		return err
	}
	defer tx.Rollback()

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

	mu.Lock()
	defer mu.Unlock()

	tx, err := db.Begin()
	if err != nil {
		log.Printf("开始事务时出错 (BatchIncrementRequestCounts): %v", err)
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
	INSERT INTO request_stats (service_name, request_count) VALUES (?, ?)
	ON CONFLICT(service_name) DO UPDATE SET request_count = request_stats.request_count + excluded.request_count;
	`)
	if err != nil {
		log.Printf("准备批量更新 request_stats 语句时出错: %v", err)
		return err
	}
	defer stmt.Close()

	for serviceName, incrementValue := range counts {
		if _, err := stmt.Exec(serviceName, incrementValue); err != nil {
			log.Printf("批量更新服务 '%s' 的请求计数时出错: %v", serviceName, err)
		}
	}

	return tx.Commit()
}

// UpdateProxyConfig 更新代理配置到数据库。
func UpdateProxyConfig(configs []config.ProxyConfig) error {
	if db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	mu.Lock()
	defer mu.Unlock()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM proxy_config")
	if err != nil {
		return err
	}

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
