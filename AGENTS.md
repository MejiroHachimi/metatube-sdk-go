# MetaTube 工作指引

## 定位与入口

这是可独立部署的 Jellyfin / Emby 元数据服务。`metatube-sdk-go` 模块名沿用上游；应用入口是 `cmd/metatube`，不要误用保留的 `cmd/server`。

先查看 `git status --short` 和相关 diff，保留已有改动。按任务读取文件，不必扫描整个 SDK：

| 任务 | 先读 |
| --- | --- |
| 配置、启动、关闭 | `cmd/metatube/main.go`、`internal/service/config.go`、`internal/service/app.go` |
| API、认证、清洗 | `internal/service/gateway.go`、`validate.go`、`clean.go`，再看 `route/` |
| 图片、缓存、并发 | `internal/service/image_work.go`、`cache.go`、`imageutil/`、`engine/image.go` |
| SQLite | `database/`、`engine/dbengine/` |
| 数据源 | 对应的 `provider/` 子目录及其测试 |
| 部署 | [部署文档](docs/deployment.md)、`Dockerfile`、`Dockerfile.heroku`、`heroku.yml` |
| API / 适配文档站 | `mkdocs.yml`、`docs/api/`、`docs/integrations/`、`scripts/docs_hooks.py` |

表中省略目录的文件与同一单元格内的首个文件同目录。

API 文档采用 Swagger UI；调用参数和返回示例直接维护在 OpenAPI 的 `example/examples` 中，不为单个实例创建页面，也不生成逐响应码的 Markdown 目录。

采集真实响应示例时，按维护者要求，不人工查看响应体，不向终端或对话输出正文。通过脚本原样写入，用结构检查和前后一致性校验验收；不要手工筛选字段或改写返回值，审查 diff 时也不要输出响应正文。

## 当前兼容约束

除非任务明确要求改变这些行为，修改时保持：

- `/v1` 插件契约、`data/error` JSON 结构；API 变更同步维护 `internal/service/openapi.json`，它随二进制嵌入。
- 分类清洗只影响响应，不批量改写数据库、标题或 Jellyfin 已有条目。
- 仅 SQLite；保留旧数组字段的序列化兼容性。
- 图片固定 WebP 质量 80；错误不缓存；缓存键区分影响输出的参数。
- 排队、SDK 并发和像素预算共同限制资源。HTTP 超时不代表底层工作已停止；正在执行的工作结束前不能提前释放额度。
- 图片下载保持插件兼容的公开访问；自定义图片 URL 必须属于该条目，不能变成任意 URL 代理。
- Docker 生产路径要求原生 WebP；本地纯 Go 回退不代表生产后端已验证。两个 Dockerfile 的入口、依赖和运行限制应保持一致。

## 验证与完成

按 [测试文档](docs/testing.md) 的变更矩阵选择检查。`make test` 是确定性服务测试；`make verify` 另外构建所有包，但不包含完整容器 CI。

不要把 `go test ./...` 当默认检查：provider、翻译等集成测试会访问真实外站或需要凭据。新增回归测试应验证可观察行为，优先使用本地 HTTP 夹具和临时 SQLite。

Go 代码修改后对相关文件运行 `gofmt`；交付前检查 `git diff --check`。文档站或 OpenAPI 修改按 [文档维护流程](docs/documentation.md) 运行 `mkdocs build --strict`，核对命令、路径和链接，无需重跑后端压测。测试通过后，除非有新修改或未解决的问题，不重复扩大验证范围。

汇报改动、实际执行的检查及结果、未验证的边界。历史覆盖率和压测记录不能作为本次测试结果。构建通过也不能替代运行时、真实抓取或插件验收。

## 提交方式

本仓库按维护者要求，完成修改和必要验证后，直接在 `main` 提交并 push 到 `origin/main`，无需再次询问。除非用户另有明确要求，不创建工作分支或 PR。推送前检查远端更新，保留已有提交和无关改动；不使用强制推送覆盖历史。

## 工具与长期任务

明确目标和验收条件后直接执行范围内工作；缺失信息影响正确性时再澄清。复杂任务的拆分与续接约定见 [harness 文档](docs/harness.md)。

配置和日志只读取任务所需内容，不输出 Token 或账号凭据。部署按本次任务已授权的目标应用执行；普通代码任务不隐含修改线上实例。已有明确授权时无需重复询问。

上游来源见 [UPSTREAM.md](UPSTREAM.md)，其中的旧 README 仅作历史参考。产品配置以 [环境变量文档](docs/configuration.md) 和实际实现为准；API 参考从 OpenAPI 生成，不复制多份配置表或额外的模型提示词文件。
