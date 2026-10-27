# Agents Quota Watcher

一个运行在本机的 Go Web 应用，用于集中查看 **Claude**、**GitHub Copilot** 的额度使用情况，以及 **DeepSeek API** 账户余额。默认仅监听本机地址，不会上传任何令牌或用量数据。

## 功能

- Claude：5 小时会话、7 天全模型和 7 天 Sonnet 使用率与重置时间。
- GitHub Copilot：当月 Premium Requests 总量、额度上限和模型明细。
- DeepSeek API：各币种当前余额、赠余额、充值余额及 API 可用状态。
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

打开 <http://127.0.0.1:48080>，浏览器会要求输入上述用户名和密码；随后添加对应服务的访问令牌。DeepSeek 使用在 DeepSeek API 平台创建的 API Key，程序通过 `GET https://api.deepseek.com/user/balance` 查询当前余额；余额只表示账户现状，不代表用量历史。Claude 的 OAuth 访问令牌可同时保存刷新令牌和到期时间；配置 Copilot 时还必须填写 GitHub 用户名和每月 credits 上限。上限默认 1500，可按计划修改；仪表盘会显示各模型的 `grossQuantity` 已消耗 AI credits 以及总和，并保留两位小数。程序直接调用 `GET /users/{username}/settings/billing/ai_credit/usage`（API 版本 `2026-03-10`），不再通过 `/user` 自动查询用户名。若要让受信任局域网中的客户端（例如 ESP32-C3）访问本机服务，需显式设置 `QUOTA_LISTEN_ADDR=0.0.0.0:48080`；该服务使用 HTTP，避免在不可信网络上传输 Basic Auth 凭据。

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

Docker Compose 默认只绑定本机回环地址。若需要让同一受信任局域网中的设备读取 `/api/usage`，可将 `QUOTA_BIND_ADDRESS` 设置为运行主机的局域网 IP 后再启动 Compose；接口继续使用 HTTP Basic Auth。不要在不可信网络上使用明文 HTTP，需通过 HTTPS 反向代理访问。

可用环境变量：

| 变量 | 默认值 | 用途 |
| --- | --- | --- |
| `QUOTA_LISTEN_ADDR` | `127.0.0.1:48080` | HTTP 监听地址 |
| `QUOTA_BIND_ADDRESS` | `127.0.0.1` | Docker Compose 主机绑定地址；仅在需要局域网访问时设置 |
| `QUOTA_CREDENTIAL_FILE` | `~/.agents-quota-watcher/credentials.json` | 本地凭据文件 |
| `QUOTA_AUTH_USERNAME` | 无，必须设置 | Web Basic Auth 用户名 |
| `QUOTA_AUTH_PASSWORD` | 无，必须设置 | Web Basic Auth 密码 |
| `CLAUDE_API_BASE_URL` | `https://api.anthropic.com` | Claude API 基础地址，便于测试或代理 |
| `GITHUB_API_BASE_URL` | `https://api.github.com` | GitHub API 基础地址，便于测试或 GitHub Enterprise |
| `DEEPSEEK_API_BASE_URL` | `https://api.deepseek.com` | DeepSeek API 基础地址，便于测试或代理 |

> 默认监听地址为 `127.0.0.1:48080`，不要将服务暴露到公网；该 Web UI 和 `/api/usage` 使用 HTTP Basic Auth 保护，仍应通过 HTTPS 或受信任的反向代理访问。

`GET /api/usage` 返回 JSON 快照。余额指标使用 `amount`（数值）、`currency`（币种）、`available`（API 余额可用状态）和 `breakdown` 中的 `amount` 表示；既有额度指标的 `used`、`limit`、`percent` 字段保持不变。此稳定接口可供轻量 HTTP/JSON 客户端使用，并继续受同一 Basic Auth 保护。

## 项目结构

```text
cmd/server/                  程序入口、配置、生命周期管理
internal/domain/             Provider 接口和通用额度领域模型
internal/app/                刷新编排、快照状态和凭据用例
internal/providers/claude/   Claude API 适配器
internal/providers/copilot/ GitHub Copilot API 适配器
internal/providers/deepseek/ DeepSeek API 余额适配器
internal/providers/          Provider 注册表
internal/store/              本地凭据持久化
internal/web/                HTTP API 和仪表盘
```

## 验证

```bash
go test ./...
go vet ./...
```
