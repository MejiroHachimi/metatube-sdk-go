# 实测：用 arbb014 搜索并刮削详情

本例实际调用了已部署的 MetaTube 服务，搜索和详情均返回 **HTTP 200**。搜索时间为 2026-10-03 15:28:01，详情时间为 15:28:32（日本时间 UTC+9）。这是当时的响应快照，不保证后续第三方站点始终可用。

| 含义 | 实测值 |
| --- | --- |
| 输入搜索词 | `arbb014` |
| 返回的影片编号 `number` | `ARBB-014` |
| 数据源 `provider` | `JAV321` |
| 数据源内部条目 `id` | `h_1092arbb00014` |

关键是先搜索得到 `provider` 和 `id`，再用这两个值请求详情。**不要把 `arbb014` 或 `ARBB-014` 直接当作 JAV321 的内部 ID。**

## 准备服务地址和 Token

本次实测服务根地址：

```sh
METATUBE_URL='https://metatube-webp-lab-c4349d-f1c18d4d478c.herokuapp.com'
```

复现时可替换为自己的服务地址。下方命令通过本地环境变量 `TOKEN` 读取该服务的 API Token；本次使用的真实密钥不会写入文档。Heroku 部署可在应用 Settings → Config Vars 中查看 `TOKEN`，不要使用 Heroku 账号 API Key。

## 第一步：搜索

实际请求：

```http
GET /v1/movies/search?q=arbb014&fallback=false
Authorization: Bearer <TOKEN>
```

| 参数 | 位置 | 本次值 | 作用 |
| --- | --- | --- | --- |
| `q` | query | `arbb014` | 按输入搜索 |
| `provider` | query | 未传 | 搜索所有已启用来源 |
| `fallback` | query | `false` | 禁用数据库结果回退；不是使用默认 true |
| `Authorization` | header | `Bearer <TOKEN>` | 服务认证 |

可复制命令：

```sh
curl --fail-with-body --get "$METATUBE_URL/v1/movies/search" \
  -H "Authorization: Bearer $TOKEN" \
  --data-urlencode 'q=arbb014' \
  --data-urlencode 'fallback=false'
```

实际响应头：

```http
HTTP 200
Content-Type: application/json; charset=utf-8
Cache-Control: no-store
```

当时返回 1 条结果。下面是**实际响应的字段摘录**：保留原字段和值，仅省略标题、页面与媒体地址；不是完整响应，也不是模拟数据。

```json
{
  "data": [
    {
      "id": "h_1092arbb00014",
      "provider": "JAV321",
      "number": "ARBB-014",
      "actors": [
        "南梨央奈"
      ],
      "release_date": "2016-07-08T00:00:00Z",
      "score": 4
    }
  ]
}
```

## 第二步：用结果中的 provider 和 id 请求详情

实际请求：

```http
GET /v1/movies/JAV321/h_1092arbb00014?lazy=false
Authorization: Bearer <TOKEN>
```

| 参数 | 位置 | 本次值 | 作用 |
| --- | --- | --- | --- |
| `provider` | path | `JAV321` | 使用搜索结果中的来源 |
| `id` | path | `h_1092arbb00014` | 使用来源内部 ID |
| `lazy` | query | `false` | 请求刷新上游元数据，而非默认优先读取已存记录 |
| `Authorization` | header | `Bearer <TOKEN>` | 同上 |

```sh
curl --fail-with-body --get \
  "$METATUBE_URL/v1/movies/JAV321/h_1092arbb00014" \
  -H "Authorization: Bearer $TOKEN" \
  --data-urlencode 'lazy=false'
```

同样返回 `HTTP 200`、`Content-Type: application/json; charset=utf-8` 和 `Cache-Control: no-store`。

以下是**详情响应的字段摘录**，保留原值及原始空字符串。为聚焦接口接入，省略 `title`、`summary`、`series`、`genres`、`homepage` 与图片/预览地址字段；完整字段定义见[接口与模型参考](reference.md)。

```json
{
  "data": {
    "id": "h_1092arbb00014",
    "provider": "JAV321",
    "number": "ARBB-014",
    "actors": [
      "南梨央奈"
    ],
    "maker": "＆RiBbON",
    "label": "",
    "director": "",
    "release_date": "2016-07-08T00:00:00Z",
    "runtime": 144,
    "score": 4
  }
}
```

搜索返回的 `data` 是数组，详情返回的 `data` 是单个对象。`label`、`director` 等字段可能为空，客户端应允许这种情况。正常读取时可省略 `lazy` 或设为 `true`；本次使用 `false` 是为了验证刷新路径。

## 同次实测中的失败响应

在上面的成功请求前，还分别尝试了以下来源限定搜索：

| 请求 query | 时间（UTC+9） | HTTP | 实际 error.message |
| --- | --- | --- | --- |
| `q=arbb014&provider=FANZA&fallback=false` | 15:27:11 | 500 | `upstream request failed` |
| `q=arbb014&provider=JavBus&fallback=false` | 15:27:56 | 404 | `Not Found` |

FANZA 的完整错误 JSON：

```json
{"error":{"code":500,"message":"upstream request failed"}}
```

JavBus 的完整错误 JSON：

```json
{"error":{"code":404,"message":"Not Found"}}
```

这只能证明当时这些请求失败，不能据此认定具体原因或影片不存在。改为不限定来源的搜索后，实际取得了 JAV321 的结果。单来源搜索失败与服务健康状态是不同层次的问题。

这里只验证了搜索与详情元数据，没有把图片下载或 Jellyfin/Emby 客户端显示算作本次实测通过项。
