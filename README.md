# Go Proxy 项目

这是一个简单的 HTTP 代理项目，解决部分场景下因网络原因无法使用个别 AI 服务的问题，使用 Go 语言和 Echo 框架构建。

## 项目简介

本项目实现了一个基本的 HTTP 代理服务器，支持通过配置文件定义多个 AI 代理规则，并集成统计功能，记录代理请求信息，通过一个简单的 Web 界面展示。

## 主要特性

*   **多规则代理**: 支持通过 YAML 配置文件定义多个上游代理目标。
*   **请求统计**: 记录每个代理规则的请求次数、成功次数、失败次数等统计信息，并存储在 SQLite 数据库中。
*   **Web 界面**: 提供一个简单的 Web 页面，用于实时查看代理规则和统计数据。
*   **优雅关停**: 支持接收系统信号，实现服务器的优雅关停。

## 项目结构

```
.
├── data/
│   ├── config.yaml       # 配置文件
│   └── stats.db          # 统计数据库
├── internal/
│   ├── config/
│   │   └── config.go     # 配置加载逻辑
│   ├── db/
│   │   └── db.go         # 数据库操作逻辑
│   ├── middleware/
│   │   └── stats.go      # 统计中间件
│   ├── proxy/
│   │   └── director.go   # 代理请求转发逻辑
│   └── routes/
│       └── routes.go     # 路由定义
├── web/
│   ├── index.html        # Web 界面 HTML
│   ├── script.js         # Web 界面 JavaScript
│   └── style.css         # Web 界面 CSS
├── go.mod                # Go 模块文件
├── go.sum                # Go 模块校验文件
└── main.go               # 项目入口文件
```

## 构建与运行

1.  **克隆项目**:
    ```bash
    git clone <项目仓库地址> # 请替换为实际的项目仓库地址
    cd go-proxy
    ```
2.  **安装依赖**:
    ```bash
    go mod tidy
    ```
3.  **配置**:
    编辑 `data/config.yaml` 文件，配置代理规则。示例配置如下：
    ```yaml
    server:
      port: "8080" # 服务器监听端口
    proxies:
      - path: "/gemini" # 匹配的请求路径前缀
        target: "https://generativelanguage.googleapis.com" # 目标地址
      - path: "/google"
        target: "https://www.google.com"
    ```
4.  **运行**:
    ```bash
    go run main.go
    ```
    服务器将在配置文件指定的端口启动。

## 配置文件 (`data/config.yaml`)

配置文件使用 YAML 格式，包含 `server` 和 `proxies` 两部分：

*   `server`:
    *   `port`: 服务器监听的端口号。
*   `proxies`: 一个代理规则列表。每个规则包含：
    *   `path`: 匹配的请求路径前缀。
    *   `target`: 请求将被转发到的目标地址。

## Web 界面

启动服务器后，访问 `http://localhost:<端口号>/` (替换 `<端口号>` 为配置文件中指定的端口) 即可访问 Web 界面，查看代理规则和实时统计数据。

## 依赖

*   [Echo](https://github.com/labstack/echo): 高性能、可扩展、低内存占用的 Go Web 框架。
*   [go-sqlite3](https://github.com/mattn/go-sqlite3): SQLite 驱动。

## 许可证

本项目采用 [MIT 许可证](LICENSE)。 # 如果有许可证文件，请取消注释并替换 LICENSE
