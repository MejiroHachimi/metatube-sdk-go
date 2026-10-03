# MetaTube Server

开箱即用的 Jellyfin / Emby 元数据后端：内置 SQLite、分类清洗、图片缓存和交互式 API 文档。

[在线文档](https://mejirohachimi.github.io/metatube-sdk-go/) · [文档站维护](docs/documentation.md)

这是可独立部署的服务应用，无需另外编写程序调用 SDK。仓库目录及 Go 模块路径中的 `metatube-sdk-go` 沿用上游命名；部署使用本仓库的 `cmd/metatube` 入口。浏览器入口 `/docs` 是交互式 API 文档，目前不提供独立的 Web 管理界面。

```sh
docker compose up --build -d
```

默认监听本机 8080 端口。浏览器打开服务的 `/docs` 查看并试用接口，`/openapi.json` 下载完整 OpenAPI 3.0 契约。Jellyfin MetaTube 插件的 Server 填写这台服务的地址。如果 Jellyfin 运行在其他机器或容器中，需要将 Compose 的端口绑定调整到它能访问的网卡，并配置 `TOKEN`。

本地默认无需密钥；公开部署请设置 `TOKEN`。Heroku 模板会自动生成它，不需要填写数据库连接信息。插件的 Token 填写服务的 `TOKEN`，它与 Heroku 账号 API Key 无关。

## 首版提供的能力

- 保持插件使用的 `/v1` 元数据、搜索、评论、图片端点和 `data/error` JSON 结构。
- 默认 SQLite 存储；Docker 命名卷保存元数据。仅支持 SQLite，可用 `DSN` 指定 SQLite 文件路径或 `file:` URI。
- 返回纯文本字段；数组清理空项、去重；默认排除 `1080p`、`Blu-ray`、`Bluray`、`Blu ray`、`蓝光`、`藍光`、`ブルーレイ`、`Blu-ray（ブルーレイ）`。
- 分类排除采用不区分大小写的完整名称匹配，清洗只发生在响应中。原始数据库记录、影片标题和已有 Jellyfin 条目不会被批量改写。
- 图片直接编码为有损 WebP，质量固定为 80（不是保证缩小 80%）。成品按图片类型和裁剪、水印参数缓存；默认 64 MiB 预算、24 小时固定 TTL、单张超过 8 MiB 不入缓存。预算包含图片、键和估算开销，不等于整个进程内存上限。
- 并发相同图片请求合并；ETag / 304、HEAD、`X-Cache`。错误响应不缓存；元数据响应 `no-store`。
- 上游图片限制 16 MiB 压缩数据、2000 万像素；最多 4 个 SDK 请求进入处理阶段；图片按原始分辨率共享默认 600 万像素额度，大于额度的图片独占处理。额外最多排队 16 个不同图片任务，队列满返回 503，等待超时返回 504。
- `/healthz` 存活检查、`/readyz` 数据库检查、可配置请求时限、SIGTERM 优雅退出。
- `/docs` 完全本地加载，不依赖第三方文档脚本或 CDN。

本仓库包含固定版本 SDK 源码，不依赖旁边的其他检出目录。来源、许可证和改动边界见 [UPSTREAM.md](UPSTREAM.md)。服务入口是 `cmd/metatube`；SDK 原入口仅作为上游参考保留。旧的 PostgreSQL DSN 和 `DATABASE_URL` 会在启动时明确报错；现有 SQLite 数据格式保持兼容。

## API 与适配

- [API 调用指南](docs/api/index.md)：认证、调用流程、图片缓存和错误处理。
- 完整接口与模型由 `internal/service/openapi.json` 自动生成到在线文档站；已部署服务的 `/docs` 支持交互式调用。
- [Jellyfin / Emby 接入](docs/integrations/jellyfin-emby.md)与[数据源适配开发](docs/integrations/providers.md)。

## 配置

环境变量、默认值、分类过滤示例和资源限制见 [配置文档](docs/configuration.md)。所有配置均可选；公开部署请设置 `TOKEN`。

## Heroku

仓库提供 `app.json`、`heroku.yml` 和兼容 Heroku 构建器的 `Dockerfile.heroku`。推荐按 [Heroku 源码部署步骤](docs/deployment.md#heroku-源码部署推荐) 创建应用、配置 Token、发布并启动 web dyno。使用私有仓库时先在 Heroku 连接相应 GitHub 账号，再选择本仓库；也可使用 CLI 的 Container Registry 发布方式。Heroku 的 dyno 通常计费，模板不自动开通数据库等付费附加服务。

- `app.json` 自动生成 `TOKEN`，启动后在应用 Config Vars 中读取并填入插件。
- `DATA_DIR=/tmp/metatube` 使用临时 SQLite 缓存；dyno 替换或重启可能丢失缓存。本版本仅支持 SQLite；需要持久化时应部署在能挂载持久卷的平台。
- 进程读取平台的 `PORT`，不硬编码监听端口。
- 默认请求时限 25 秒，先于常见的平台路由超时返回可读错误。
- 图片缓存只在当前进程内，重启后为空；多实例不共享。需要跨实例缓存时，再引入对象存储或共享缓存。

完整 Docker / CLI 部署与验证步骤见 [docs/deployment.md](docs/deployment.md)。

## 开发与验证

需要 Go 1.26.0+，Docker 构建固定使用 Go 1.26.8。Docker 使用动态链接和原生 libwebp；本地未安装动态库时可以使用纯 Go 回退，但资源占用不同。

```sh
make run
make build
make test
```

代理开发从 [AGENTS.md](AGENTS.md) 开始；面向 GPT-6 Astra 的任务组织、续接和当前验证缺口见 [harness 文档](docs/harness.md)。

按改动选择检查及复现命令见 [测试文档](docs/testing.md)。普通服务测试使用本地夹具，外站集成测试单独执行；原生 WebP 与 Heroku Eco 的历史性能记录见 [压测文档](docs/native-webp-testing.md)。

## 许可证

Apache-2.0。保留 SDK 原始 LICENSE 与源码声明。
