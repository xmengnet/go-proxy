package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"go-proxy/pkg/config"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
)

type ReverseProxy struct {
	proxy *httputil.ReverseProxy
}

func NewReverseProxy(cfg config.ProxyConfig) *ReverseProxy {
	targetURL, _ := url.Parse(cfg.Target)
	log.Printf("Creating reverse proxy for %s", cfg.Target)
	// 创建一个反向代理
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	// 处理路径
	pathPrefix := cfg.Path
	// 自定义 Director 函数来修改请求头
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.Host = targetURL.Host // 显式设置 Host 请求头
		req.URL.Path = strings.TrimPrefix(req.URL.Path, pathPrefix)
		log.Printf("Forwarding request to %s%s", req.URL.Host, req.URL.RequestURI()) // 记录转发的请求
	}
	return &ReverseProxy{proxy: proxy}
}

func (p *ReverseProxy) Handler(c echo.Context) error {
	// 在请求开始时记录基本信息
	log.Printf("Received request: %s %s from %s", c.Request().Method, c.Request().URL.RequestURI(), c.Request().RemoteAddr)

	// 处理请求
	p.proxy.ServeHTTP(c.Response(), c.Request())

	// 在请求结束后记录状态码
	log.Printf("Request completed: %s %s, status: %d", c.Request().Method, c.Request().URL.RequestURI(), c.Response().Status)

	return nil
}
