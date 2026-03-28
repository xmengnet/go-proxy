package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// ========================================
	// 第一类：HTTP 请求指标
	// ========================================

	// HttpRequestsTotal 各代理服务的请求总数
	HttpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "goproxy_http_requests_total",
			Help: "各代理服务的 HTTP 请求总数",
		},
		[]string{"service", "method", "status_code"},
	)

	// HttpRequestDuration 请求响应时间分布（秒）
	HttpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "goproxy_http_request_duration_seconds",
			Help:    "HTTP 请求响应时间分布（秒）",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		},
		[]string{"service", "method"},
	)

	// HttpResponseSize 响应体大小分布（字节）
	HttpResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "goproxy_http_response_size_bytes",
			Help:    "HTTP 响应体大小分布（字节）",
			Buckets: prometheus.ExponentialBuckets(100, 10, 7), // 100B ~ 100MB
		},
		[]string{"service"},
	)

	// ========================================
	// 第二类：代理层指标
	// ========================================

	// UpstreamErrorsTotal 上游服务错误计数
	UpstreamErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "goproxy_upstream_errors_total",
			Help: "上游服务错误计数",
		},
		[]string{"service", "error_type"},
	)

	// ActiveRequests 当前正在处理的并发请求数
	ActiveRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "goproxy_active_requests",
			Help: "当前正在处理的并发请求数",
		},
		[]string{"service"},
	)

	// ========================================
	// 第三类：内部组件指标
	// ========================================

	// StatsChannelUsage 统计通道当前使用量
	StatsChannelUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "goproxy_stats_channel_usage",
			Help: "统计通道当前使用量",
		},
	)

	// StatsChannelDrops 统计通道满导致的丢弃次数
	StatsChannelDrops = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "goproxy_stats_channel_drops_total",
			Help: "统计通道满导致的丢弃次数",
		},
	)

	// StatsBatchProcessTotal 批量处理执行次数
	StatsBatchProcessTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "goproxy_stats_batch_process_total",
			Help: "批量处理执行次数",
		},
	)

	// DbErrorsTotal 数据库操作错误次数
	DbErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "goproxy_db_errors_total",
			Help: "数据库操作错误次数",
		},
		[]string{"operation"},
	)
)

// Register 注册所有自定义 Prometheus 指标
func Register() {
	prometheus.MustRegister(
		HttpRequestsTotal,
		HttpRequestDuration,
		HttpResponseSize,
		UpstreamErrorsTotal,
		ActiveRequests,
		StatsChannelUsage,
		StatsChannelDrops,
		StatsBatchProcessTotal,
		DbErrorsTotal,
	)
}
