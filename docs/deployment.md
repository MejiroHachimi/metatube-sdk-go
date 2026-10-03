# 部署和验收

## Docker（默认路径）

```sh
docker compose up --build -d
docker compose ps
curl --fail http://127.0.0.1:8080/readyz
```

Compose 自动创建元数据卷。单纯 `docker compose down` 保留卷；不要使用 `down -v`，除非确实要删除数据库。

公网部署将 `TOKEN` 通过平台 secret 或未跟踪的 `.env` 注入；默认 Compose 仅映射宿主机 `127.0.0.1`，由本机反向代理提供 HTTPS。不要把真实密钥写进仓库。

## Heroku Container Registry

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
