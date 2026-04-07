# AIChatMatrix

一个基于 Go 的多智能体讨论矩阵项目，支持在网页端创建房间、配置 AI 角色、启动/停止对话引擎，并通过 WebSocket 实时查看消息流。

## 功能特性

- 多房间管理：创建、查看、更新、删除聊天室
- 多智能体协作：在房间内添加/移除 AI 角色
- 模型提供商配置：支持 OpenAI 兼容接口（自定义 `base_url` / `model` / `api_key`）
- Persona（人设）管理：可复用角色预设
- Template（模板）管理：可复用房间模板
- 对话控制：启动、停止、重启房间引擎
- 实时通信：通过 WebSocket 推送消息与状态
- 房间能力：克隆房间、导出记录、系统指令注入等

## 技术栈

- Go `1.21`
- HTTP 服务：标准库 `net/http`
- WebSocket：`github.com/gorilla/websocket`
- UUID：`github.com/google/uuid`
- 前端：原生 HTML/CSS/JavaScript（位于 `web/`）

## 项目结构

```text
.
├── cmd/main.go               # 程序入口
├── internal/
│   ├── api/                  # HTTP / WebSocket 路由与处理器
│   ├── chat/                 # 聊天引擎核心逻辑
│   ├── config/               # 配置加载与持久化
│   ├── models/               # 数据模型定义
│   └── store/                # 内存状态存储
├── web/                      # 静态前端页面
├── config.json               # 运行时配置文件
├── go.mod
└── README.md
```

## 快速开始

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 启动服务

```bash
go run ./cmd/main.go
```

默认监听端口为 `8080`。也可以通过环境变量覆盖：

```bash
PORT=9090 go run ./cmd/main.go
```

### 3. 打开页面

浏览器访问：

- `http://localhost:8080`（或你设置的端口）

## 配置说明（`config.json`）

服务启动时会读取根目录下的 `config.json`。如果文件不存在，将使用默认配置。

常见字段：

- `port`：默认端口
- `providers`：模型提供商列表
- `personas`：人设列表
- `folders`：人设分组
- `templates`：房间模板

示例：

```json
{
  "port": "8080",
  "providers": [
    {
      "id": "provider-1",
      "name": "OpenAI-Compatible",
      "base_url": "https://api.example.com/v1/chat/completions",
      "api_key": "YOUR_API_KEY",
      "model": "gpt-4o-mini",
      "max_tokens": 2048
    }
  ],
  "personas": [],
  "folders": [],
  "templates": []
}
```

## 常用 API（概要）

- Provider
  - `GET /api/providers`
  - `POST /api/providers`
  - `PUT /api/providers/{id}`
  - `DELETE /api/providers/{id}`
- Persona
  - `GET /api/personas`
  - `POST /api/personas`
  - `PUT /api/personas/{id}`
  - `DELETE /api/personas/{id}`
- Template
  - `GET /api/templates`
  - `POST /api/templates`
  - `PUT /api/templates/{id}`
  - `DELETE /api/templates/{id}`
- Room
  - `GET /api/rooms`
  - `POST /api/rooms`
  - `GET /api/rooms/{id}`
  - `PUT /api/rooms/{id}`
  - `DELETE /api/rooms/{id}`
  - `POST /api/rooms/{id}/start`
  - `POST /api/rooms/{id}/stop`
  - `POST /api/rooms/{id}/restart`
  - `GET /api/rooms/{id}/messages`
  - `GET /api/rooms/{id}/export`
  - `POST /api/rooms/{id}/clone`
  - `GET/WS /api/rooms/{id}/ws`

## 开发建议

- 优先通过前端页面完成配置与联调
- 修改后可直接使用 `go run ./cmd/main.go` 验证
- 若需要扩展新路由，建议从 `internal/api/router.go` 开始

## 验收脚本（推荐）

新增 `acceptance_room.sh`，用于快速验证两件事：

1. 裁判输出中的 `[NO_OP]` 不会泄漏到房间消息
2. 裁判输出 `STOP_DISCUSSION` 时房间会自动停止

使用方式：

```bash
PROVIDER_ID=<你的provider_id> ./acceptance_room.sh
```

可选参数：

- `BASE_URL`：默认 `http://localhost:8080`
- `TIMEOUT_SEC`：默认 `120`

## License

如需开源发布，请在此补充具体许可证（例如 MIT / Apache-2.0）。
