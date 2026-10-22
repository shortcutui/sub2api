# 自有 fork 的开发与测试部署

仓库为 `shortcutui/sub2api`。`dev` 用于开发和测试，自己的 `production` 用于保存验证通过的版本；`ranxi2001/sub2api` 的 `production` 只是同步来源。先将上游改动合入 `dev` 验证，再将自己的 `dev` 合入自己的 `production`，不要用上游分支直接覆盖自己的修改。

## 开发与构建

本地调试使用源码运行：前端 `pnpm --dir frontend dev`，后端在 `backend/` 中配置独立开发数据库后执行 `go run ./cmd/server`。远程测试使用 Docker 镜像，以验证实际交付的程序。

推送 `dev` 或 `production` 会触发 `.github/workflows/fork-images.yml`，执行更新服务测试、前端类型检查及构建、后端编译、全新数据库初始化和持久化验证，通过后发布 `linux/amd64` 镜像：

- `ghcr.io/shortcutui/sub2api:dev-<提交号前12位>`
- `ghcr.io/shortcutui/sub2api:production-<提交号前12位>`

工作流摘要提供镜像地址和固定摘要。默认分支包含该工作流后，也可在 Actions 页面手动选择 `dev` 或 `production` 构建。工作流使用内置 `GITHUB_TOKEN` 发布 GHCR，不需要服务器 SSH 密钥，也不会自动部署。首次创建的 GHCR 包可能是私有的；服务器拉取时需要包读取权限，或由仓库所有者在包设置中改为公开。

镜像以 `container` 类型构建，管理后台显示镜像版本提示并禁用应用内二进制更新和回退。源码或正式二进制 Release 的更新源为 `shortcutui/sub2api`。

## 全新测试环境

新机器安装 Docker Engine 和 Compose 插件。将本目录的 `compose.yml`、`.env.example` 复制到一个独立目录，例如 `/opt/sub2api-test`。

```sh
cd /opt/sub2api-test
cp .env.example .env
chmod 600 .env
```

填写 `.env` 中的实际镜像、管理员邮箱和密码。分别执行 `openssl rand -hex 32`，生成数据库密码、Redis 密码、JWT 密钥和 TOTP 密钥。不要复用旧服务器的配置或数据。

```sh
docker compose --env-file .env -f compose.yml config --quiet
docker compose --env-file .env -f compose.yml pull
docker compose --env-file .env -f compose.yml up -d --wait --wait-timeout 240
docker compose --env-file .env -f compose.yml ps
```

`BIND_HOST=0.0.0.0` 时可通过 `http://服务器IP:8080` 访问；需要 SSH 隧道时设为 `127.0.0.1`。PostgreSQL 和 Redis 仅通过 Compose 网络访问。三个数据卷按 Compose 项目名隔离，PostgreSQL 18 的实际数据目录位于挂载的 `/var/lib/postgresql` 下。

## 更新与后续迁移

修改 `.env` 中的 `SUB2API_IMAGE` 为另一个已验证的镜像标签或摘要，然后只更新应用：

```sh
docker compose --env-file .env -f compose.yml pull sub2api
docker compose --env-file .env -f compose.yml up -d --no-deps --wait sub2api
```

测试期间保持全新数据。以后迁移旧机器时，先用备份副本演练数据库升级，并单独安排切换。应用升级可能执行数据库迁移，因此回退镜像前需要确认数据库兼容性，不能把镜像回退等同于数据库回退。
