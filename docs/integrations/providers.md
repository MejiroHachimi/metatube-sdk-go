# 数据源适配开发

数据源适配器负责将站点数据转换成统一模型；应用层负责参数验证、鉴权、响应清洗、图片缓存和资源控制。修复一个来源时，优先修改对应 provider，避免把站点特例扩散到公共网关。

## 内置演员来源

| 来源 | 默认可用条件 | 当前映射的主要字段 |
| --- | --- | --- |
| AV-LEAGUE | 无需配置 | 姓名、别名、图片、生日、身高、血型、罩杯、三围、出道日期；上游“出身”沿用 `nationality` 字段，不一定是国家 |
| Gfriends | 无需配置 | 按姓名精确匹配头像，不提供人物生理资料 |
| ThePornDBActor | 配置 [API Token](../configuration.md#演员数据源) | 姓名、简介、别名、图片、生日、身高、国籍、罩杯、三围；当前不提供血型 |

`/v1/actors/search` 不指定 `provider` 时搜索所有已启用来源。搜索结果是概要；用结果中的 `provider` 与 `id` 请求详情，才能取得生日等字段。人物资料按来源分别返回，不自动合并；日语来源的详情会附加 Gfriends 头像。Gfriends 禁用或返回错误时保留原来源图片。

详情默认先读 SQLite；`lazy=false` 重新抓取。Gfriends 单独使用图片索引缓存，不走人物资料数据库。字段完整度取决于来源和人物条目：未知数值为 0、未知文本为空，日期可能为零日期；不要据此推断生日或其他资料。完整参数和返回示例见 [API](../api/index.md)。

## 代码入口

| 位置 | 职责 |
| --- | --- |
| `provider/provider.go` | Provider、MovieProvider、ActorProvider 及可选能力接口 |
| `provider/factory.go` | 工厂注册 |
| `provider/<name>/` | 具体站点实现与测试 |
| `engine/register.go` | 空白导入，使 provider 的 init 注册生效 |
| `engine/init.go` | 实例化、应用配置，跳过 priority ≤ 0 的 provider |
| `model/` | 元数据与搜索结果模型 |
| `internal/envconfig/` | MT_* 环境变量解析 |

可参照 `provider/javbus/javbus.go` 中的接口断言、`New` 工厂和 `provider.Register(Name, New)`。注册工厂后还需要在 `engine/register.go` 导入该包。

## 新增或修复适配器

1. 确认来源需要影片、演员或两种能力，实现对应 Provider 接口。搜索、评论、图片 Fetcher 等能力按需实现。
2. 明确站点 URL、ID 解析和规范化规则；返回稳定的 provider 名称和 ID，不把影片展示编号与内部 ID 混用。
3. 将结果映射到 `model.MovieInfo` / `model.ActorInfo` 等统一模型，提供正确的原始图片地址。不要把 API Token 放入模型或公开 URL。
4. 注册工厂并导入包，确认启动后 `/v1/providers` 中可见。检查 priority，值不大于 0 时该来源会被禁用。
5. 为解析、缺失字段和异常响应补本地 fixture 测试；需要实际网络的测试单独执行，再验证通过应用网关的返回结果。

SDK 部分接口没有完整的请求上下文取消支持。增加长耗时操作时检查超时和资源释放，不假定客户端断开就会终止解码或原生编码。

## Provider 配置

环境变量以双下划线分隔 provider 名称与配置键：

```sh
MT_PROVIDER_JAVBUS__TIMEOUT=5s
MT_MOVIE_PROVIDER_JAVBUS__PRIORITY=995
```

- `MT_PROVIDER_<NAME>__<KEY>`：通用配置。
- `MT_MOVIE_PROVIDER_<NAME>__<KEY>` / `MT_ACTOR_PROVIDER_<NAME>__<KEY>`：类型专属配置，覆盖通用的同名键。
- 通用键包含 `priority`、`timeout` 和 `proxy`；后两项要求适配器实现相应 setter。其他键由 `ConfigSetter` 实现决定。
- 名称与键不区分大小写；含连字符的 provider 名称可在环境变量中使用下划线。

这些变量在进程初始化时读取，更新后需重启服务。proxy 等配置可能进入 SDK 日志，不在代理 URL 中嵌入账号密码。

## 验证边界

改动应通过[服务验证](../testing.md)，相关 provider 的集成测试只在所需网络与凭据可用时单独运行。例如 `go test ./provider/javbus` 会访问真实站点，不属于离线解析测试。

如果新增公共接口或改变参数、错误状态、模型结构，同步维护 `internal/service/openapi.json` 并构建文档站。完整 API 参考会自动更新；不要手写第二份参数表。
