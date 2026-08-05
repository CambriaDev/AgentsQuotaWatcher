# Agents Quota Watcher

一个运行在本机的 Go Web 应用，用于集中查看 **Claude** 和 **GitHub Copilot** 的额度使用情况。默认监听 `127.0.0.1:8080`，不会上传任何令牌或用量数据。

## 功能

- Claude：5 小时会话、7 天全模型和 7 天 Sonnet 使用率与重置时间。
- GitHub Copilot：当月 Premium Requests 总量、额度上限和模型明细。
- 多账户卡片：同一 Provider 可添加多个账户；为每个账户设置别名（例如“工作 Copilot”“个人 Copilot”）以便区分。未添加账户时不会显示空卡片。
- 本地凭据管理：访问令牌保存在 `~/.agents-quota-watcher/credentials.json`，目录为 `0700`、文件为 `0600`。
- 自动刷新：启动时刷新一次，之后每分钟刷新；界面也可手动刷新。
- 可扩展 Provider 架构：Provider 不依赖 Web 层。未来接入 Google Antigravity 时，实现 `internal/domain.Provider` 并在 `cmd/server/main.go` 注册即可。

## 运行

需要 Go 1.22 或更高版本：

```bash
QUOTA_AUTH_USERNAME=admin \
QUOTA_AUTH_PASSWORD='choose-a-strong-password' \
go run ./cmd/server
```

打开 <http://127.0.0.1:8080>，浏览器会要求输入上述用户名和密码；随后分别为 Claude 和 Copilot 输入可访问对应额度 API 的访问令牌。Claude 的 OAuth 访问令牌可同时保存刷新令牌和到期时间；配置 Copilot 时还必须填写 GitHub 用户名和每月 credits 上限。上限默认 1500，可按计划修改；仪表盘会显示各模型的 `grossQuantity` 已消耗 AI credits 以及总和，并保留两位小数。程序直接调用 `GET /users/{username}/settings/billing/ai_credit/usage`（API 版本 `2026-03-10`），不再通过 `/user` 自动查询用户名。

### Docker Compose

先在运行环境中设置网页登录凭据：

```bash
export QUOTA_AUTH_USERNAME=admin
export QUOTA_AUTH_PASSWORD='choose-a-strong-password'
```

```bash
docker compose up -d --build
```

打开 <http://127.0.0.1:48080>，浏览器会要求输入上述用户名和密码。认证信息会持久化到项目根目录的 `./cert/credentials.json`；`cert` 目录应妥善保管，避免提交到版本控制。

可用环境变量：

| 变量 | 默认值 | 用途 |
| --- | --- | --- |
| `QUOTA_LISTEN_ADDR` | `127.0.0.1:8080` | HTTP 监听地址 |
| `QUOTA_CREDENTIAL_FILE` | `~/.agents-quota-watcher/credentials.json` | 本地凭据文件 |
| `QUOTA_AUTH_USERNAME` | 无，必须设置 | Web Basic Auth 用户名 |
| `QUOTA_AUTH_PASSWORD` | 无，必须设置 | Web Basic Auth 密码 |
| `CLAUDE_API_BASE_URL` | `https://api.anthropic.com` | Claude API 基础地址，便于测试或代理 |
| `GITHUB_API_BASE_URL` | `https://api.github.com` | GitHub API 基础地址，便于测试或 GitHub Enterprise |

> 不要将默认监听地址暴露到公网；该 Web UI 使用 HTTP Basic Auth 保护，仍应通过 HTTPS 或受信任的反向代理访问。

## 项目结构

```text
cmd/server/                  程序入口、配置、生命周期管理
internal/domain/             Provider 接口和通用额度领域模型
internal/app/                刷新编排、快照状态和凭据用例
internal/providers/claude/   Claude API 适配器
internal/providers/copilot/ GitHub Copilot API 适配器
internal/providers/          Provider 注册表
internal/store/              本地凭据持久化
internal/web/                HTTP API 和仪表盘
```

## 验证

```bash
go test ./...
go vet ./...
```
