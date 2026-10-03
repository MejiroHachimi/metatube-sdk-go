# MetaTube Server

开箱即用的 Jellyfin / Emby 元数据后端：内置 SQLite、分类清洗、图片缓存和交互式 API 文档。

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

## API 契约

```json
{"data":{"id":"example001","genres":["Drama"]}}
```

```json
{"error":{"code":400,"message":"quality must be between 1 and 100"}}
```

上述成功示例仅展示字段片段，完整字段与类型见 `/docs`。图片成功响应为 `image/webp`；图片错误也是 JSON。错误码与 HTTP 状态一致，不返回堆栈、数据库地址或上游错误原文。

| GET 路径 | 用途 | Token |
| --- | --- | --- |
| `/` | 服务名与版本 | 无 |
| `/healthz`, `/readyz` | 健康检查 | 无 |
| `/v1/providers` | 可用数据源 | 无 |
| `/v1/movies/search?q=...` | 搜索影片 | 配置后需要 |
| `/v1/movies/{provider}/{id}` | 影片详情 | 配置后需要 |
| `/v1/actors/search?q=...` | 搜索演员 | 配置后需要 |
| `/v1/actors/{provider}/{id}` | 演员详情 | 配置后需要 |
| `/v1/reviews/{provider}/{id}` | 评论 | 配置后需要 |
| `/v1/db/version` | 数据库版本 | 配置后需要 |
| `/v1/images/{primary,thumb,backdrop}/{provider}/{id}` | 图片，亦支持 HEAD | 无，兼容插件图片下载 |
| `/docs`, `/openapi.json` | API 文档 | 无 |

认证使用 `Authorization: Bearer <TOKEN>`，不要将 Token 放入查询字符串。所有查询参数的范围、默认值、返回模型和错误状态在 OpenAPI 中列明。旧客户端的 `quality=1..100` 参数仍接受，但统一输出质量 80，且不影响缓存键。未知参数和重复参数返回 400。搜索结果为空时返回 404，保持原插件行为；没有分页接口。

图片 `url` 参数只接受该条目元数据中已有的图片地址，避免把服务当作任意 URL 代理。水印仅支持内置 `zimu.png`、`u.png`、`uc.png`。图片缓存与 SDK 元数据缓存相互独立；`lazy=false` 更新元数据，不会立即清除已生成图片，图片会在 TTL 到期或进程重启后重建。

首版不暴露上游的翻译接口、模块调试列表和 redirect 快捷入口。插件的自动翻译应设为 Disabled；如果依赖自定义远程水印或这些高级功能，应先验证需求再切换。第三方站点的可达性、付费访问和反爬限制不能由部署模板保证。

## 配置：全部可选

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `8080` | HTTP 端口，自动遵循 Heroku 注入端口 |
| `BIND` | 空（所有网卡） | 监听地址；Compose 默认只映射宿主机回环 |
| `DATA_DIR` | 本地 `./data`；容器 `/data` | SQLite 所在目录 |
| `DSN` | 空 | SQLite 文件路径或 `file:` URI；为空时使用 `DATA_DIR/metadata.db` |
| `TOKEN` | 空 | 本地无需认证；公网请设置，Heroku 自动生成 |
| `EXCLUDED_GENRES` | 上述 8 个分类名 | 英文逗号分隔；显式设为空禁用过滤；设置时替换完整列表 |
| `IMAGE_CACHE_MB` | `64` | 成品缓存预算 0–1024 MiB；0 禁用存储 |
| `IMAGE_CACHE_TTL` | `24h` | 固定缓存有效期，读取不会续期 |
| `REQUEST_TIMEOUT` | `25s` | HTTP 等待时限，超时返回 JSON 504 |
| `MAX_CONCURRENT` | `4` | 同时执行的 SDK 请求数，1–64 |
| `IMAGE_PIXEL_BUDGET` | `6000000` | 同时处理图片的原始像素总预算，100 万–2000 万；超出预算的单张图片独占处理 |
| `IMAGE_QUEUE_SIZE` | `16` | 额外等待的不同图片任务数，0–64；重复请求合并，不重复占队列 |
| `REQUIRE_NATIVE_WEBP` | 本地未设置；Docker 为 `1` | 为 `1` 时要求原生 libwebp 可用，否则启动失败 |
| `GOMEMLIMIT` | Docker 为 `192MiB` | Go 内存软目标，不包含 libwebp 原生分配，不是进程内存硬上限 |

如需额外排除 4K 和 720P，设置完整列表，例如：

```sh
EXCLUDED_GENRES='1080p,720p,4K,Blu-ray,Bluray,Blu ray,蓝光,藍光,ブルーレイ,Blu-ray（ブルーレイ）'
```

过滤发生在抓取返回后，不能自动删除 Jellyfin 库里已经存在的标签，需要刷新元数据或使用插件的分类替换任务。网站提供的“1080P”分类不等同于实际本地视频分辨率。

SDK 的 `MT_*` provider 配置仍可使用，见上游项目；通常不必设置。网络请求使用 SDK 的提供者适配器；SDK 未完整支持请求上下文取消，因此 HTTP 超时后某些上游工作可能短暂继续；原生编码也不能中途强制终止。运行中的任务直到实际结束才释放并发和像素额度。排队中的图片会在超时或所有客户端断开时取消，不会提前解码。

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

测试覆盖真实 SDK + SQLite、Token、响应清洗、数据库原文保留、PNG 下载到 WebP 输出、缓存命中、并发合并、LRU/TTL、参数拒绝、错误不缓存、ETag、超时、请求并发上限和图片大小限制。测试图片来自本地 HTTP 服务，不依赖外部网站。SDK 的 provider 集成测试依赖真实站点；这些测试不会在普通服务 CI 中执行。

覆盖率、容器端到端测试及复现命令见 [docs/testing.md](docs/testing.md)。

原生 libwebp、分辨率加权队列和 Heroku Eco 实测结果见 [docs/native-webp-testing.md](docs/native-webp-testing.md)。

## 许可证

Apache-2.0。保留 SDK 原始 LICENSE 与源码声明。
