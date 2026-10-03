# MetaTube Server

可直接部署的 Jellyfin / Emby 元数据后端，提供影片、演员、评论和图片 API。内置 SQLite、分类清洗、WebP 图片缓存和服务内的交互式 API 文档。

## 从这里开始

| 目标 | 文档 |
| --- | --- |
| 启动应用 | [Docker / Heroku 部署](deployment.md) |
| 调整端口、认证和缓存 | [环境变量](configuration.md) |
| 配置媒体服务器插件 | [Jellyfin / Emby 接入](integrations/jellyfin-emby.md) |
| 编写 API 客户端 | [Swagger API 参考](api/index.md)（参数、响应示例与模型） |
| 增加或修复数据源 | [适配开发](integrations/providers.md) |

## 最快启动

在仓库根目录运行：

```sh
docker compose up --build -d
curl --fail http://127.0.0.1:8080/readyz
```

打开本机服务的 `http://127.0.0.1:8080/docs` 试用 API。Compose 默认仅映射宿主机回环地址；媒体服务器位于其他机器或容器时，按[接入文档](integrations/jellyfin-emby.md)配置网络与 Token。

## 两种文档入口

- 本站由 MkDocs 和 GitHub Pages 提供，用于查阅、部署和开发，不运行 MetaTube 后端。
- 每个 MetaTube 服务自己的 `/docs` 可直接调用该服务；`/openapi.json` 描述该部署版本的接口。

本站的 Swagger UI 直接展示仓库中的同一份 OpenAPI 契约。若已部署版本较旧，请优先查看对应服务的契约。

当前提供后端服务与 API 文档，没有独立的 Web 管理后台。SQLite 在 Docker 卷中可持久化，在 Heroku 上为临时缓存；抓取结果仍取决于第三方站点的可达性。
