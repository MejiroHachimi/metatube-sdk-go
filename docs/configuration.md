# 环境变量

所有配置均可选。默认排除分类为 `1080p`、`Blu-ray`、`Bluray`、`Blu ray`、`蓝光`、`藍光`、`ブルーレイ`、`Blu-ray（ブルーレイ）`。

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `8080` | HTTP 端口，自动遵循 Heroku 注入端口 |
| `BIND` | 空（所有网卡） | 监听地址；Compose 默认只映射宿主机回环 |
| `DATA_DIR` | 本地 `./data`；Docker `/data`；Heroku 镜像 `/tmp/metatube` | SQLite 所在目录 |
| `DSN` | 空 | SQLite 文件路径或 `file:` URI；为空时使用 `DATA_DIR/metadata.db` |
| `TOKEN` | 空 | 本地无需认证；公网请设置，仅通过 Heroku app.json 模板创建时自动生成 |
| `EXCLUDED_GENRES` | 上述 8 个分类名 | 英文逗号分隔；显式设为空禁用过滤；设置时替换完整列表 |
| `IMAGE_CACHE_MB` | `64` | 成品缓存预算 0–1024 MiB；0 禁用存储 |
| `IMAGE_CACHE_TTL` | `24h` | 固定缓存有效期，读取不会续期 |
| `REQUEST_TIMEOUT` | `25s` | HTTP 等待时限，超时返回 JSON 504 |
| `MAX_CONCURRENT` | `4` | 同时执行的 SDK 请求数，1–64 |
| `METADATA_QUEUE_SIZE` | `16` | 0–64；不同元数据任务总上限为 `MAX_CONCURRENT` 加本值，含运行和排队；重复请求合并 |
| `MAX_WAITING_REQUESTS` | `128` | 1–4096；等待任务结果的 HTTP 请求总数上限，含元数据与未命中缓存的图片，共享同一任务的调用方分别计数 |
| `IMAGE_PIXEL_BUDGET` | `6000000` | 同时处理图片的原始像素总预算，100 万–2000 万；超出预算的单张图片独占处理 |
| `IMAGE_QUEUE_SIZE` | `16` | 额外等待的不同图片任务数，0–64；重复请求合并，不重复占队列 |
| `REQUIRE_NATIVE_WEBP` | 本地未设置；Docker 为 `1` | 为 `1` 时要求原生 libwebp 可用，否则启动失败 |
| `GOMEMLIMIT` | Docker 为 `192MiB` | Go 内存软目标，不包含 libwebp 原生分配，不是进程内存硬上限 |

分类排除和去重按完整名称匹配，忽略大小写、Unicode NFKC 等价写法（如全角/半角）及连续空白。例如 `１０８０Ｐ` 匹配 `1080p`，`Blu　 ray` 匹配 `Blu ray`；`Not 1080P` 不会被 `1080p` 规则删除。归一化只用于比较，保留项沿用首次出现时的写法和顺序，不改写标题、人物名、制作商、厂牌、系列或图片地址。

如需额外排除 4K 和 720P，设置完整列表，例如：

```sh
EXCLUDED_GENRES='1080p,720p,4K,Blu-ray,Bluray,Blu ray,蓝光,藍光,ブルーレイ,Blu-ray（ブルーレイ）'
```

过滤发生在抓取返回后，不能自动删除 Jellyfin 库里已经存在的标签，需要刷新元数据或使用插件的分类替换任务。网站提供的“1080P”分类不等同于实际本地视频分辨率。

## 单用户批量请求

约 16 路同时请求可先使用默认配置：SDK 最多处理 4 个任务，元数据最多接纳 20 个不同任务（含运行和排队），图片也有独立的任务接纳上限。两类任务共用这 4 个处理名额，不是各自启动 4 个。`MAX_CONCURRENT` 限制的是 SDK 调用，全源搜索内部还会并行查询多个 provider，不等于对外 HTTP 连接总数。

相同路径、规范化后的查询参数和认证信息的元数据请求只共享进行中的任务，完成后不缓存响应；相同图片沿用现有的任务合并及图片缓存。不同 Token 的元数据请求不会共享结果。单个调用方断开不影响其他等待者；所有调用方离开后取消该任务。

排队和执行合计受 `REQUEST_TIMEOUT` 限制，后来加入共享任务不会重置时限。排队取消或超时的任务不再进入 SDK；已开始且不支持取消的上游调用、图片编码必须实际结束后才释放处理名额。

队列或等待人数达到上限返回 503，并带 `Retry-After: 1`；超时返回 504。客户端应采用有限次数的退避重试并加少量随机延迟，避免立即重复提交整批任务。健康检查和已命中缓存的图片不占等待名额。增加队列长度不会提高处理速度，也不会让超过请求时限的批次自动完成；25 秒以上的持久化后台任务需要另行设计任务接口。

SDK 的 `MT_*` provider 配置见[数据源适配](integrations/providers.md#provider-配置)。网络请求使用 SDK 的提供者适配器；SDK 未完整支持请求上下文取消，因此 HTTP 超时后某些上游工作可能短暂继续；原生编码也不能中途强制终止。运行中的任务直到实际结束才释放并发和像素额度。排队中的图片会在超时或所有客户端断开时取消，不会提前解码。

## 演员数据源

默认构建启用 AV-LEAGUE 和 Gfriends，无需额外开关。ThePornDBActor 同样内置，设置以下凭据后自动启用，无需 `experimental` 构建标签：

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `MT_ACTOR_PROVIDER_THEPORNDBACTOR__ACCESS_TOKEN` | 空 | ThePornDB 的 API Token；未配置时不参与搜索，也不出现在已启用来源列表中 |

更新环境变量后重启服务。来源优先级默认为 AV-LEAGUE / ThePornDBActor 1000、Gfriends 999；`MT_ACTOR_PROVIDER_AV_LEAGUE__PRIORITY=0` 或 `MT_ACTOR_PROVIDER_GFRIENDS__PRIORITY=0` 可分别禁用来源。ThePornDB 的影片来源仍属于实验构建，不影响默认演员功能。
