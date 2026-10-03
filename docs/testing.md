# 测试与覆盖率

2026-10-03 本地验证结果：下列自动化测试、无 CGo 构建及容器端到端测试通过。

| 包 | 修改前语句覆盖率 | 修改后语句覆盖率 |
| --- | ---: | ---: |
| `internal/service` | 83.9% | 93.5% |
| `imageutil` | 29.3% | 93.1% |
| `database` | 未测量 | 100.0% |
| `engine/dbengine` | 未测量 | 93.6% |

修改前后均按各包自身测试统计。以上四个包合计覆盖率为 93.7%，不代表整个仓库或外部 provider 的覆盖率。

## 复现

需要 Go 1.26+；容器测试另需 Python 3 和本机 Docker。

```sh
go test -race -coverprofile=coverage.out ./internal/service ./imageutil ./database ./engine/dbengine
go tool cover -func=coverage.out
CGO_ENABLED=0 go test -tags=nodynamic ./imageutil ./internal/service
CGO_ENABLED=0 go build ./...
docker build -t metatube:test .
python3 scripts/smoke.py metatube:test
```

CI 执行这些检查，并将 `coverage.out` 保存为 `test-coverage` 附件。
需要代理 CA 的构建环境可使用可选 BuildKit secret：

```sh
docker build --secret id=proxy_ca,src=/path/to/trusted-ca-bundle.pem -t metatube:test .
```

## 覆盖内容

- 真实 SQLite 的建表、读写、更新、查询、关闭重开及原有数组数据格式兼容；拒绝 PostgreSQL 连接配置。
- 配置默认值与边界值、Token 认证、元数据清洗、原始记录不被改写、非法参数和错误响应。
- 实际 PNG/JPEG/WebP 解码、裁剪、缩放、水印和有损 WebP 编码；输出使用独立解码器检查，编码质量固定为 80。
- 缓存容量、LRU、固定 TTL、禁用缓存、超大图片、相同请求合并、ETag/304、HEAD、超时和并发上限。
- 容器脚本通过真实 HTTP 请求读取本地 PNG 测试图，检查 WebP 响应和缓存，并重建容器验证 SQLite 卷持久化、缓存重置及优雅退出。测试专用容器和卷自动清理。

## 验证边界

这些测试不依赖真实第三方站点，也没有启动 Jellyfin/Emby 客户端。实际站点抓取、反爬限制和客户端显示兼容性需要另行验证。

PostgreSQL 的连接、迁移和查询分支已删除。`lib/pq` 仅用于保留旧 SQLite 数组字段的序列化兼容；`gorm.io/driver/postgres` 在模块图中仍是 `gorm.io/datatypes` 上游测试的间接依赖，不是本服务可用的数据库后端。
