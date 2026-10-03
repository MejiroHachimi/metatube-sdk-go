# Jellyfin / Emby 接入

本服务保留 MetaTube 插件使用的 `/v1` 端点和 `data/error` 结构。以下说明针对插件后端接入；插件安装包和版本由相应插件项目维护。自动化测试尚不包含真实 Jellyfin/Emby 客户端。

## 连接配置

1. 按[部署文档](../deployment.md)启动服务，确认 `/readyz` 返回 200。
2. 在媒体服务器中安装适用于该平台的 MetaTube 插件，打开其配置页。
3. Server 填服务根地址，例如 `https://your-app.example`，不要附加 `/v1` 或 `/docs`。
4. Token 填服务的 `TOKEN`，不是 Heroku 账号 API Key。将插件自动翻译设为 Disabled。
5. 保存后用少量条目验证搜索、元数据刷新与封面，再启动批量任务。

## 网络地址怎么选

| 运行方式 | Server 地址选择 |
| --- | --- |
| 媒体服务器与 MetaTube 都运行在宿主机 | 可使用宿主机的 `127.0.0.1:8080` |
| 两者位于同一个 Docker 网络 | 使用 MetaTube 容器服务名和容器端口，例如 `http://metatube:8080` |
| 媒体服务器在其他机器或 Docker 网络 | 使用它能够访问的宿主机地址或 HTTPS 反向代理域名 |
| Heroku | 使用 Dashboard / `heroku apps:info` 返回的真实 HTTPS Web URL |

容器内的 `127.0.0.1` 指该容器自身。默认 Compose 仅绑定宿主机回环地址，其他机器不能直接连接；按实际网卡调整映射并设置 `TOKEN`，或通过反向代理提供 HTTPS。不要将 GitHub Pages 文档站地址填入 Server。

## 与上游 SDK 的差异

| 行为 | 当前应用 |
| --- | --- |
| 翻译 | 不开放翻译端点，插件自动翻译应关闭 |
| 图片 | 固定质量 80 的 WebP；旧 quality 参数接受但不改变质量 |
| 水印 | 仅内置 badge，不支持远程水印 URL |
| 分类 | 响应中清洗并排除配置中的完整分类名；不会改写原始数据库记录 |
| 存储 | 仅 SQLite，Heroku 上不保证缓存持久化 |
| 高级入口 | 不开放模块调试列表和 redirect 快捷入口 |

## 分类与 Tag 的字段映射

后端详情没有独立的 `tags` 字段。[上游插件的映射逻辑](https://github.com/metatube-community/jellyfin-plugin-metatube/blob/f7c1f336fc2bd3b35b82af7c6e69da09a196e3d2/Jellyfin.Plugin.MetaTube/Providers/MovieProvider.cs#L127)如下；不同插件版本或分支可能有所调整。

| API 详情字段 | 插件写入媒体库 |
| --- | --- |
| `genres` | 分类（Genres），可经过插件的分类替换规则 |
| `series`、`maker`、`label` | 各自非空时添加为 Tag |
| `maker` | 还会写入制作商（Studio） |

`label` 只表示厂牌，不代表全部 Tag。即使它为空，`maker` 或 `series` 仍可生成 Tag。

分类清洗不会自动删除媒体库已有标签。调整排除配置后，需要刷新对应元数据或使用插件自身的分类替换任务；“1080P”分类与本地文件真实分辨率不是同一字段。

## 分类规则与现成工具

后端的 `EXCLUDED_GENRES` 适合多个客户端共用的排除名单，匹配方式见[环境变量](../configuration.md)。分辨率、载体等分类是否保留取决于媒体库用途；独占发行、限时发行等信息也并非对所有用户都无用，因此默认名单不自动扩张。

仅需调整 Jellyfin/Emby 显示时，可直接使用插件的 **Enable genre substitution** 和 **Genre substitution table**。一行一条 `原分类=新分类`，等号右侧留空表示删除；分类数组按完整名称、不区分大小写匹配。例如以下是可选规则，不会自动启用：

```text
ハイビジョン=
Sci-Fi=科幻
Science Fiction=科幻
```

规则只作用于分类，不会清空 `maker`、`label`、`series`。后端过滤先于插件替换执行，被后端排除的分类不会再交给插件处理。[插件规则实现](https://github.com/metatube-community/jellyfin-plugin-metatube/blob/f7c1f336fc2bd3b35b82af7c6e69da09a196e3d2/Jellyfin.Plugin.MetaTube/Helpers/SubstitutionTable.cs#L51)不需要另行安装过滤框架；不同插件版本的设置名称可能略有不同。

| 工具 | 适用范围与当前选择 |
| --- | --- |
| [bluemonday](https://github.com/microcosm-cc/bluemonday) | 已用于 HTML 清洗；不负责判断哪些分类有用 |
| [Go x/text / norm](https://pkg.go.dev/golang.org/x/text/unicode/norm) | 已有依赖，用于分类匹配键的 Unicode 归一化，无需新增框架 |
| [Stash](https://github.com/stashapp/stash) | 完整的媒体管理应用，具备标签别名与层级；可参考其分类管理设计，不作为本服务的过滤库直接嵌入 |
| [CEL-Go](https://github.com/cel-expr/cel-go) | 可嵌入的表达式引擎，适合以后按来源、字段等组合条件编写规则；当前精确名单与替换表尚不需要它，也不自带分类词库 |

## 接入验收

- 从媒体服务器所在网络访问 `/readyz` 和 `/v1/providers`。
- 未带 Token 请求 `/v1/db/version` 返回 401，正确 Token 返回 200。
- 搜索真实条目，检查 provider、ID、标题和清洗后的分类。
- 刷新封面，确认客户端支持 WebP，第二次相同图片请求可以命中缓存。
- 批量刷新时限制并发，对 503/504 做有限重试。Eco 休眠后首个请求可能需要等待唤醒。

健康检查正常不代表第三方来源全部可用。遇到抓取失败时先用服务自己的 `/docs` 复现，区分插件连接、认证、参数和上游站点问题。
