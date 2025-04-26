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
├── docker-compose.yaml   # Docker Compose 配置
├── Dockerfile            # Docker 构建文件
├── go.mod                # Go 模块文件
├── go.sum                # Go 模块校验文件
├── LICENSE               # 项目许可证
├── main.go               # 项目入口文件
├── README.md             # 项目说明文件
├── pkg/                  # 公共包目录
│   ├── config/           # 配置相关包
│   │   └── config.go     # 配置加载逻辑
│   └── proxy/            # 请求转发相关包
│       └── director.go   # 请求转发逻辑
├── vercel.json           # Vercel 配置
├── api/
│   └── index.go          # Vercel Serverless 函数入口
├── data/
│   ├── config-sample.json # JSON 格式示例配置文件
│   ├── config-sample.yaml # YAML 格式示例配置文件
│   └── config.yaml       # 实际使用的配置文件
├── internal/
│   ├── config/
│   │   └── config.go     # 配置加载逻辑
│   ├── db/
│   │   └── db.go         # 数据库操作逻辑
│   ├── middleware/
│   │   └── stats.go      # 统计中间件
│   └── routes/
│       └── routes.go     # 路由定义
└── public/
    ├── index.html        # Web 界面 HTML
    ├── script.js         # Web 界面 JavaScript
    └── style.css         # Web 界面 CSS
```

## 构建与运行

1.  **克隆项目**:
    ```bash
    git clone https://github.com/xmengnet/go-proxy.git
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

请确保 `data/config.yaml` 文件存在并配置正确。如果使用 Docker Compose，配置文件会被挂载到容器内。如果直接使用 `docker run`，你需要手动将配置文件挂载到容器的 `/app/data/config.yaml` 路径。

### Docker Hub 镜像

项目镜像已发布到 Docker Hub：`xmengnet/go-proxy`。可以直接拉取并运行：

```bash
docker pull xmengnet/go-proxy
docker run -d -p 8080:8080 -v data:/app/data --name go-proxy xmengnet/go-proxy
```

同样需要注意配置文件的挂载。

## 配置文件 (`data/config.yaml`)

配置文件使用 YAML 格式，包含 `server` 和 `proxies` 两部分，把项目的`config-sample.yaml` 文件重命名为`config.yaml`即可：

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

## 部署到 Vercel

本项目也可以部署到 Vercel 作为无服务器函数。在 Vercel 环境中，统计功能（数据库和统计中间件）将被禁用，`/api/stats` 接口将只返回代理节点信息（不包含请求次数）。

### Vercel 配置

在 Vercel 项目设置中，需要配置 `PROXIES_CONFIG` 环境变量来定义代理规则。

*   `PROXIES_CONFIG`: 包含一个 JSON 数组的字符串，其中每个元素是一个代理配置对象。格式类似于 `data/config-sample.json` 文件中的 `proxies` 数组部分。

    示例 `PROXIES_CONFIG` 环境变量值：
    ```json
    [
      {"path":"/gemini","target":"https://generativelanguage.googleapis.com"},
      {"path":"/google","target":"https://www.google.com"}
    ]
    ```

### 注意事项

*   由于 Vercel 无服务器环境的限制，数据库和请求统计功能不可用。
*   `/api/stats` 接口在 Vercel 环境下仅返回代理节点的 `path` 和 `target` 信息，`access_count` 将固定为 0。
*   请注意，Vercel 由于不支持 go 语言的 Flush() 函数，导致流式输出不可用，因此不建议使用 Vercel 部署。

## 部署到 Render

可以将本项目部署为 Render 的 Web Service。

### 构建和部署

1.  在 Render 控制台创建一个新的 Web Service。
2.  连接你的 Git 仓库。
3.  配置构建命令 (Build Command)，例如 `go build -o go-proxy main.go`。
4.  配置启动命令 (Start Command)，例如 `./go-proxy`.
5.  配置环境变量同 Vercel 配置。

### 配置

可以通过以下方式配置代理规则：

1.  **使用 `data/config.yaml`**: 如果将 `data` 目录及其内容包含在部署中，项目将读取 `data/config.yaml` 文件。这是推荐的方式，与本地运行一致。
2.  **使用环境变量**: 所有部署方式都支持设置环境变量。你可以通过环境变量来配置需要代理的连接，但是需要 JSON 格式，可以参照 `config/config-sample.json` 配置。


## 许可证

本项目采用 [MIT 许可证](LICENSE)。
