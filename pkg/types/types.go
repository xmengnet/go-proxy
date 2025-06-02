package types

// RequestStat 定义一个结构体来传递统计数据
// 这个结构体现在被 middleware 和 db 包共享
type RequestStat struct {
	ServiceName string
	Host        string
	RequestURI  string
	StatusCode  int
}
