# 部署和验收

## Docker（默认路径）

```sh
docker compose up --build -d
docker compose ps
curl --fail http://127.0.0.1:8080/readyz
```

Compose 自动创建元数据卷。单纯 `docker compose down` 保留卷；不要使用 `down -v`，除非确实要删除数据库。

公网部署将 `TOKEN` 通过平台 secret 或未跟踪的 `.env` 注入；默认 Compose 仅映射宿主机 `127.0.0.1`，由本机反向代理提供 HTTPS。不要把真实密钥写进仓库。

## Heroku 源码部署（推荐）

Heroku 源码部署使用 `heroku.yml` 指定的 `Dockerfile.heroku`，兼容不支持 BuildKit 的平台构建器。它与默认 Dockerfile 使用相同的服务入口和原生 WebP 运行依赖，不包含测试夹具。默认 Dockerfile 仍用于本地 Docker / Compose 和下述 Container Registry 发布流程。

### 1. 创建应用和配置

需要 Heroku 账号、可用的 dyno 套餐和 Heroku CLI。以下命令中的 `YOUR-STAGING-APP` 替换为自己的全局唯一应用名；先部署测试应用，验证后再切换插件。

```sh
heroku login
heroku create YOUR-STAGING-APP --stack container --region us
```

如果使用已有应用，先确认应用名称，并将它的 stack 设为 `container`：

```sh
heroku stack:set container --app YOUR-STAGING-APP
```

在该应用 Dashboard → Settings → Config Vars 设置：

| 变量 | 设置 |
| --- | --- |
| `TOKEN` | 用密码管理器生成的随机长密钥；稍后填入插件 |
| `DATA_DIR` | `/tmp/metatube` |

无需设置数据库附加服务、`DATABASE_URL` 或 `DSN`。不要手动固定 `PORT`，服务读取 Heroku 注入的端口。仅通过 `app.json` 部署模板创建应用时会自动生成 Token；手动创建、GitHub 部署和 CLI 发布均应检查 Config Vars，不能假定模板已执行。

### 2. 构建并发布

在 Dashboard → Deploy 连接 GitHub 账号，选择本仓库或自己的 fork，然后选择包含 `Dockerfile.heroku` 与新版 `heroku.yml` 的分支，点击 Deploy Branch。私有仓库需要授予 Heroku 对该仓库的访问权限。

也可以从本地已提交的代码通过 Heroku Git 发布：

```sh
heroku git:remote --app YOUR-STAGING-APP
git push heroku HEAD:main
```

未提交的本地修改不会随 Git 发布。构建日志应显示 `Building web (Dockerfile.heroku)`，随后完成镜像推送和 release。只有构建成功后才继续启动。

自动化也可调用 Heroku Platform API：向 `/apps/APP/builds` 提交 `source_blob.url`（Heroku 能直接下载的源码 `.tar.gz`）与 `source_blob.version`（提交 SHA），轮询返回的 build ID 直到 `status=succeeded`，再调整 formation。建议使用固定提交的归档地址，而不是可变化的分支地址。私有源码应使用授权下载地址，不能把长期账号密钥写进 URL 或仓库。

### 3. 启动和验证

在 Dashboard → Resources 启用一个 web dyno，并选择账号可用的类型。已有 Eco 套餐时，可用 CLI：

```sh
heroku ps:scale web=1:eco --app YOUR-STAGING-APP
heroku ps --app YOUR-STAGING-APP
heroku apps:info --app YOUR-STAGING-APP
```

使用 `apps:info` 返回的真实 Web URL，域名可能带随机后缀，不要自行拼接。打开该地址的 `/docs`；`/healthz`、`/readyz`、`/openapi.json` 和 `/v1/providers` 应返回 200。在 `/docs` 输入 Config Vars 中的 Token 后，可试用受保护接口；`/v1/db/version` 无 Token 应返回 401，带正确 Bearer Token 应返回 200。

启动日志应包含 `WebP backend: native libwebp` 和 `MetaTube listening`。Jellyfin/Emby 插件的 Server 填应用根地址（不附加 `/docs` 或 `/v1`），Token 填 `TOKEN`，自动翻译设为 Disabled。

### 4. 更新、日志和停止

更新时重新 Deploy Branch，或再次执行 `git push heroku HEAD:main`。发布后重新检查 `/readyz` 和认证，不需要手动重建数据库。

```sh
heroku logs --tail --app YOUR-STAGING-APP
heroku releases --app YOUR-STAGING-APP
# 测试结束后停止；需要恢复时将 web=0 改为 web=1
heroku ps:scale web=0 --app YOUR-STAGING-APP
```

Eco 会休眠，首次访问可能需要等待唤醒；运行消耗账号 Eco 时长。停止 dyno 不等于取消付费套餐。SQLite 位于临时磁盘，重启、部署或实例替换可能清空缓存；本版本不支持 PostgreSQL。需要持久化应选择可挂载持久卷的平台。

### 常见问题

| 现象 | 检查方式 |
| --- | --- |
| 构建报 `the --mount option requires BuildKit` | 确认部署分支的 `heroku.yml` 指向 `Dockerfile.heroku` |
| 应用不可访问、H14 | 检查构建/release 是否成功，以及 web 数量是否为 1 |
| 应用启动失败、H10 | 查看日志；检查是否残留 `DATABASE_URL`、PostgreSQL DSN 或不可写的 `DATA_DIR` |
| API 返回 401 | 使用应用的 `TOKEN`，而不是 Heroku 账号 API Key；Header 为 `Authorization: Bearer <TOKEN>` |
| 健康检查正常，但抓取失败 | 检查具体数据源的可达性、认证和反爬限制；健康检查不代表所有第三方站点可用 |

## Heroku Container Registry（本地构建）

此路径发布到调用者指定的应用。先用新测试应用验证，再切换 Jellyfin；不覆盖现有生产应用。

```sh
heroku create YOUR-STAGING-APP --stack container --region us
heroku container:login
heroku container:push web --app YOUR-STAGING-APP
```

通过 Heroku Dashboard 为该应用设置 `TOKEN` 和 `DATA_DIR=/tmp/metatube`。CLI container 流程不会自动读取 app.json 的 secret generator，因此这一步不可省略。不要将账号 `HEROKU_API_KEY` 用作插件 Token。

```sh
heroku container:release web --app YOUR-STAGING-APP
heroku ps:scale web=1 --app YOUR-STAGING-APP
```

选择 dyno 类型会产生相应费用。使用 Heroku 部署模板创建时可由 app.json 自动生成插件 Token。Registry 发布与 Platform API 是不同认证目标；云环境若通过代理注入凭据，必须分别配置目标域名，不能假定仅绑定 api.heroku.com 的凭据能用于 registry.heroku.com。

## 验收清单

1. `/healthz` 和 `/readyz` 返回 200，且分别返回 `data.status=ok/ready`。
2. `/v1/providers` 非空；`/docs` 与 `/openapi.json` 可加载。
3. 配置 TOKEN 后，无凭据访问 `/v1/db/version` 返回 401；带正确 Bearer 返回 200。
4. 从一个已启用的数据源选取真实条目，查询详情并确认不包含排除分类；不要把“服务启动成功”等同于“所有来源均可抓取”。
5. 同一图片与参数连续请求，首次 MISS，随后 HIT；带返回的 ETag 应得到 304。失败图片返回 no-store，且不会进入缓存。
6. Docker 下重建容器后，SQLite 卷中的数据保留；Heroku 临时磁盘不作此保证。
7. 将 Jellyfin 指向测试服务，关闭插件自动翻译，验证搜索、元数据刷新和封面，再切换常用配置。

## 扩展边界

本地 LRU 只缓存成品 WebP（固定质量 80），不缓存错误。多副本缓存独立；需要长期图片存储时，可进一步接入 S3 兼容对象存储。不要通过关闭 TLS 校验解决抓取失败。
