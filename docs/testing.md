# 测试与验证

## 按变更选择验证

所有命令在仓库根目录执行。需要 Go 1.26.0+；`-race` 需要 CGO 和 C 编译器，容器测试另需 Docker 与 Python 3。先运行 `go version`、`docker info` 确认工具可用。`make run` / `make build` 使用 `cmd/metatube`；`make build` 的纯 Go 构建与生产容器的原生 WebP 路径不同。

| 改动 | 验证 |
| --- | --- |
| 仅说明文档 | 核对相对链接、命令和实际配置；`git diff --check`，无需重跑后端压测 |
| 服务、认证、清洗、SQLite | `make test`；针对新增或修复行为运行对应回归用例 |
| 包接口、依赖、跨包修改 | `make verify`；它运行 `make test` 和 `go build ./...` |
| API 参数、状态码、响应模型 | 服务回归测试；同步 `internal/service/openapi.json`，运行时核对 `/openapi.json` 与 `/docs` |
| 图片编码、缓存、队列、取消 | 服务测试 + 下方纯 Go 回退、原生后端、容器冒烟；资源控制变化再跑限资源压测 |
| 默认 Docker 镜像/入口 | 构建生产镜像并执行 `scripts/smoke.py` |
| Heroku 镜像/部署配置 | 按 [部署文档](deployment.md) 验证平台构建、启动和 HTTPS；默认镜像通过不能代替 Heroku 构建 |
| 特定 provider / 翻译适配器 | 单独运行对应集成测试，明确网络、凭据及外站限制 |

普通服务检查不使用 `go test ./...`：仓库中的部分测试访问真实站点或需要外部服务密钥。下列服务测试使用本地 HTTP 夹具和临时 SQLite；首次安装 Go 依赖仍可能需要网络。

## 可复现命令

确定性服务检查及纯 Go 回退：

```sh
make verify
go tool cover -func=coverage.out
CGO_ENABLED=0 go test -tags=nodynamic ./imageutil ./internal/service
CGO_ENABLED=0 go build ./...
```

原生 WebP 与生产容器检查：

```sh
docker build --target test -t metatube:native-tests .
docker build -t metatube:test .
python3 scripts/smoke.py metatube:test
```

冒烟脚本会调用宿主机 `go` 构建图片 HTTP 夹具，自动创建并清理专用容器/卷。不要把它换成真实用户数据库。

完整检查范围以 [.github/workflows/test.yml](../.github/workflows/test.yml) 为准。`make verify` 不包含原生镜像、纯 Go 回退测试或容器冒烟；CI 额外保存 `test-coverage` 附件并运行限资源负载检查。负载测试只针对 `loadtest` 镜像创建的一次性测试服务，命令与实验条件见 [原生 WebP 测试记录](native-webp-testing.md#复现)。

需要代理 CA 时，可对上面每条 `docker build` 增加 `--secret id=proxy_ca,src=/path/to/trusted-ca-bundle.pem`；运行中的外部 HTTPS 请求还需相应运行时信任配置，见执行环境指引。不要关闭 TLS 校验。

## 历史覆盖率快照

以下为 2026-10-03 已记录的本地结果，不表示后续提交或当前工作区已经通过检查。原生后端与 Heroku Eco 压测条件见 [native-webp-testing.md](native-webp-testing.md)。

| 包 | 修改前语句覆盖率 | 修改后语句覆盖率 |
| --- | ---: | ---: |
| `internal/service` | 83.9% | 93.7% |
| `imageutil` | 29.3% | 90.6% |
| `database` | 未测量 | 100.0% |
| `engine/dbengine` | 未测量 | 93.6% |

修改前后均按各包自身测试统计。以上四个包合计覆盖率为 93.4%，不代表整个仓库或外部 provider 的覆盖率。

## 覆盖内容

- 真实 SQLite 的建表、读写、更新、查询、关闭重开及原有数组数据格式兼容；拒绝 PostgreSQL 连接配置。
- 配置默认值与边界值、Token 认证、元数据清洗、原始记录不被改写、非法参数和错误响应。
- 实际 PNG/JPEG/WebP 解码、裁剪、缩放、水印和有损 WebP 编码；输出使用独立解码器检查，编码质量固定为 80。
- 缓存容量、LRU、固定 TTL、禁用缓存、超大图片、相同请求合并、ETag/304、HEAD、超时和并发上限。
- 容器脚本通过真实 HTTP 请求读取本地 PNG 测试图，检查 WebP 响应和缓存，并重建容器验证 SQLite 卷持久化、缓存重置及优雅退出。测试专用容器和卷自动清理。

## 验证边界

这些测试不依赖真实第三方站点，也没有启动 Jellyfin/Emby 客户端。实际站点抓取、反爬限制和客户端显示兼容性需要另行验证。

PostgreSQL 的连接、迁移和查询分支已删除。`lib/pq` 仅用于保留旧 SQLite 数组字段的序列化兼容；`gorm.io/driver/postgres` 在模块图中仍是 `gorm.io/datatypes` 上游测试的间接依赖，不是本服务可用的数据库后端。
