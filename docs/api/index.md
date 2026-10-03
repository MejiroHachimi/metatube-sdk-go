# API 调用指南

API 根地址是你部署的 MetaTube 服务，例如 `https://your-app.example`。不要使用本站 GitHub Pages 地址作为插件 Server 或 API Base URL。

完整的参数、默认值、状态码和模型见[自动生成的参考](reference.md)，也可[下载 OpenAPI JSON](openapi.json)。

## 认证与响应

配置 `TOKEN` 后，元数据、搜索、评论和数据库版本接口使用 `Authorization: Bearer <TOKEN>`。不要把 Token 放入查询字符串。未配置 Token 时这些接口也可公开访问。

健康检查、数据源列表、图片与文档接口保持公开。成功 JSON 使用 `data`，失败使用 `error`：

```json
{"data":{"id":"example001","genres":["Drama"]}}
```

```json
{"error":{"code":400,"message":"quality must be between 1 and 100"}}
```

成功示例仅展示部分字段。图片成功响应为 `image/webp`，错误仍为 JSON；不要仅根据请求路径把所有返回内容写成图片。

## 端点概览

| 路径 | 用途 | 配置 Token 后 |
| --- | --- | --- |
| `/` | 服务名称、版本 | 公开 |
| `/healthz`、`/readyz` | 存活、数据库就绪检查 | 公开 |
| `/v1/providers` | 当前启用的数据源 | 公开 |
| `/v1/movies/search`、`/v1/actors/search` | 搜索 | 需要认证 |
| `/v1/movies/{provider}/{id}`、`/v1/actors/{provider}/{id}` | 元数据详情 | 需要认证 |
| `/v1/reviews/{provider}/{id}` | 评论 | 需要认证 |
| `/v1/db/version` | 数据库版本 | 需要认证 |
| `/v1/images/{kind}/{provider}/{id}` | 图片，kind 为 primary、thumb 或 backdrop | 公开 |
| `/docs`、`/openapi.json` | 当前服务的交互文档与契约 | 公开 |

使用 GET；图片还支持 HEAD。健康检查与文档支持 HEAD 的细节见完整参考。

## 最小调用流程

先将 `METATUBE_URL` 设置为自己的服务根地址；受保护请求从本地环境中的 `TOKEN` 读取密钥。

```sh
METATUBE_URL='http://127.0.0.1:8080'
curl --fail-with-body "$METATUBE_URL/readyz"
curl --fail-with-body "$METATUBE_URL/v1/providers"
curl --fail-with-body --get "$METATUBE_URL/v1/movies/search" \
  -H "Authorization: Bearer $TOKEN" \
  --data-urlencode 'q=EXAMPLE-001'
```

`EXAMPLE-001` 是占位值，需要替换为真实搜索词。可使用 `provider` 限定来源；`fallback` 默认 true。搜索结果为空返回 404，没有分页接口。

从搜索结果中取出数据源标识和条目 ID，对路径段进行 URL 编码后请求详情。`lazy=true` 默认优先读数据库；`lazy=false` 刷新上游元数据。影片编号不一定等于 provider 内部 ID，不能直接混用。

## 图片与缓存

- 图片统一输出有损 WebP，质量固定 80。旧 `quality=1..100` 参数仍接受，但不会改变输出质量或缓存键。
- 使用 `X-Cache: MISS/HIT` 观察缓存，使用 `ETag` 和 `If-None-Match` 获取 304；HEAD 无响应体。错误不缓存。
- 图片 `url` 必须与该条目元数据中的一个图片 URL 完全相符；自定义远程水印不支持，仅支持 `zimu.png`、`u.png`、`uc.png`。
- 元数据响应为 `no-store`；刷新元数据不会立即清除成品图片缓存。图片缓存到期或进程重启后重建。
- 演员的常规图片入口使用 `primary`；其他图片种类的适用条件见对应接口。

裁剪参数和范围以自动参考为准。未知参数、重复参数和越界值返回 400。

## 客户端错误处理

| 状态 | 处理 |
| --- | --- |
| 400 | 修正参数或图片 URL，不原样重试 |
| 401 | 核对服务 Token 与 Bearer Header |
| 404 | 检查 provider/ID；搜索无匹配也返回 404 |
| 503 | 减少并发，延迟并有限重试 |
| 504 | 请求超时，有限退避重试；底层工作可能仍在结束过程中 |

客户端设置请求时限，并限制批量元数据/图片刷新并发。图片额度和 `GOMEMLIMIT` 不等于完整进程的硬内存上限。

本服务不开放上游 SDK 的翻译、模块调试和 redirect 快捷接口。需要接入旧客户端时见[插件兼容说明](../integrations/jellyfin-emby.md)。
