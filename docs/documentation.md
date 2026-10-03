# 文档站维护

本站使用 MkDocs Material，源码位于 `docs/`，导航与主题位于 `mkdocs.yml`。GitHub Pages 只托管静态文档，不承载 MetaTube API。

## 本地预览与构建

需要 Python 3.12+。在仓库根目录执行：

```sh
python3 -m venv .venv-docs
.venv-docs/bin/python -m pip install -r docs/requirements.txt
.venv-docs/bin/python -m mkdocs serve
```

打开终端输出的本地地址。提交前构建：

```sh
.venv-docs/bin/python -m mkdocs build --strict
git diff --check
```

`site/` 和 `.venv-docs/` 不提交。API 页面使用 `overrides/api.html` 中的 Swagger UI，直接读取由 `scripts/docs_hooks.py` 发布的 `internal/service/openapi.json`。构建会按 OpenAPI 内容生成带哈希的文件名供 Swagger 加载，避免沿用旧版缓存；固定地址 `api/openapi.json` 保留用于下载。参数示例写入 OpenAPI 的 `example`，返回示例写入对应响应的 `content.application/json.examples`；不为单个请求新增教程页，也不再将响应码和模型展开为 Markdown 目录。

Swagger UI 5.33.1 的 JS/CSS 与许可证保存在 `docs/assets/swagger-ui/`，无需 CDN；版本与来源见其中的 `VERSION.txt`。本站禁用 Try it out，在线调用仍使用实际服务的 `/docs`。修改其他 Markdown 页面时直接编辑 `docs/`，新增页面同步更新 `mkdocs.yml` 导航。

接口默认折叠；展开后显示参数和 200 示例，其他状态通过“展开其他响应”查看。较长的补充说明可使用 MkDocs 的 `??? info "标题"` 折叠块，避免占满正文。

## GitHub Pages 自动发布

`.github/workflows/docs.yml` 在 `main` 的文档、页面模板、站点配置、生成脚本或 OpenAPI 发生变化时运行，也支持手动 `workflow_dispatch`。流程为安装固定版本的文档依赖 → 严格构建 → 上传 Pages artifact → 部署到 `github-pages` 环境。

仓库 Settings → Pages → Source 应设为 **GitHub Actions**。构建 job 只读仓库；部署 job 使用 `pages: write` 和 `id-token: write`，无需配置额外长期 Token。按仓库约定直接提交并 push 到 `main`，无需创建 PR 或发布分支。

Fork 到其他仓库时，要启用该仓库的 Pages、允许 Actions 运行，并修改 `mkdocs.yml` 中的 `site_url`、`repo_url` 和 `repo_name`，同时更新文档中的仓库链接。

## 排查

| 问题 | 检查 |
| --- | --- |
| 严格构建失败 | 查看失效链接、缺失导航项或 OpenAPI 引用；修正文档再提交 |
| Pages 部署返回 404 / site not found | 确认已启用 Pages，Source 为 GitHub Actions |
| workflow 没有运行 | 确认提交位于 main 且命中路径过滤，或从 Actions 手动运行 Documentation |
| API 调用失败 | 这是静态文档站；改为访问实际服务的 `/docs` |
| 页面版本落后 | 查看当前提交对应 workflow 的 deploy 结果，再检查 Pages 地址 |
