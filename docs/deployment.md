# 部署指南

本文档面向单学校生产部署。系统一套实例只允许初始化一所学校，所有业务数据通过 `school_id` 隔离。

## 1. 组件与端口

| 组件 | 默认地址 | 说明 |
|---|---|---|
| Next.js 前端 | `http://localhost:3000` | 生产使用 `pnpm build && pnpm start` |
| Go API | `http://localhost:8080` | `cmd/api`，启动时默认自动执行数据库迁移 |
| PostgreSQL 16 | `localhost:5432` | 唯一业务持久化存储，需要 pgcrypto |
| Redis 7 | `localhost:6379` | 仅登录/注册限流与短期缓存 |
| OCR sidecar | `127.0.0.1:8001` | 仅回环或内网访问，禁止暴露公网 |

## 2. 环境变量

| 变量 | 要求 |
|---|---|
| `APP_ENV` | 生产必须为 `production`，启用 Secure Cookie |
| `HTTP_ADDR` | 默认 `:8080` |
| `DATABASE_URL` | 指向生产 PostgreSQL 业务库，建议独立低权限账号 |
| `REDIS_URL` | 指向 Redis，不保存业务数据 |
| `OCR_URL` | 可选。指向内网 OCR sidecar，例如 `http://127.0.0.1:8001`；留空时不注册 OCR 接口，AI 工作台不受影响 |
| `JWT_SECRET` | 至少 32 字节的高强度随机值，使用密钥管理系统注入 |
| `CORS_ORIGIN` | 前端完整源，例如 `https://exam.school.edu.cn` |
| `TRUST_PROXY` | 默认 `false`。仅当 API 位于可信反向代理之后时设为 `true`，限流将使用 `X-Forwarded-For` / `X-Real-IP`；直接暴露 API 时必须保持 `false`，否则限流身份可被伪造 |
| `AUTO_MIGRATE` | 默认 `true`。API 启动时自动执行版本化迁移；使用独立迁移账号或由部署流水线单独迁移时设为 `false` |
| `AI_BASE_URL` / `AI_API_KEY` / `AI_MODEL` | 可选。OpenAI 兼容模型服务；未配置时使用本地确定性引擎 |
| `AI_SETTINGS_SECRET` | 可选。加密学校设置中的厂商 API Key；留空回退到 `JWT_SECRET` |
| `AI_UPLOAD_DIR` | AI 对话附件落盘目录，默认 `./data/ai-uploads`。生产必须使用持久化卷并纳入备份 |
| `CLOUD_UPLOAD_DIR` | 学校云盘文件落盘目录，默认 `./data/cloud-drive`。生产必须使用持久化卷并纳入备份 |
| `EMBEDDING_BASE_URL` / `EMBEDDING_API_KEY` / `EMBEDDING_MODEL` | 可选。云盘资料向量化使用的 OpenAI 兼容 `/embeddings` 服务；留空时使用内置本地哈希向量，离线可用 |
| `EMBEDDING_LOCAL_DIMENSIONS` | 本地哈希向量维度，默认 `256`，范围 64-1024。切换向量模型后必须在云盘页面重建索引 |
| `NEXT_PUBLIC_API_URL` | 构建时注入的 API 地址，修改后必须重新构建前端 |

禁止把 `.env`、访问令牌、密码或模型缓存提交到 Git。

## 3. 启动顺序

```bash
docker compose up -d                # PostgreSQL、Redis、OCR
cd backend
go run cmd/api/main.go              # 启动时自动迁移，或编译后由 systemd 托管
cd ../frontend
pnpm install --frozen-lockfile
pnpm build
pnpm start
```

API 启动时会先对 `DATABASE_URL` 执行 `migrations/` 下的版本化迁移，全部成功后才开始监听端口；迁移失败会直接退出，避免旧表结构对外服务。多实例滚动发布时由 PostgreSQL advisory lock 保证只有一个实例执行迁移。若生产环境使用独立低权限迁移账号，可设置 `AUTO_MIGRATE=false`，并在发布流水线中先执行 `go run cmd/migrate/main.go`，再启动 API。

生产建议使用进程管理器（systemd、Supervisor 或容器编排）托管 API 与前端，并为 PostgreSQL 数据卷和 OCR 模型卷配置独立持久化。

## 4. TLS 与反向代理

- 生产必须由反向代理（Nginx、Caddy 等）终止 TLS，并将前端与 API 暴露为预期的同源或跨源地址。
- `APP_ENV=production` 时认证 Cookie 带 `Secure`、`HttpOnly`、`SameSite=Strict`。
- `CORS_ORIGIN` 必须与浏览器地址栏中的前端源完全一致；所有修改请求会校验 `Origin`。
- 不要在反向代理日志中记录认证头、Cookie 或密码。

Nginx 示例（前端与 API 分别转发）：

```nginx
server {
    listen 443 ssl;
    server_name exam.school.edu.cn;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /api/ {
        client_max_body_size 25m;
        proxy_request_buffering off;
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

若按上述同域部署，`NEXT_PUBLIC_API_URL=https://exam.school.edu.cn`、`CORS_ORIGIN=https://exam.school.edu.cn`，并设置 `TRUST_PROXY=true`。否则所有请求会以反向代理自身的 IP 参与限流，导致全校共用同一份额度。启用 `TRUST_PROXY` 后必须确保 8080 端口只对反向代理开放，不能直接暴露公网。

## 5. 数据安全与备份

- 每日备份 PostgreSQL：`pg_dump -Fc school_db > backup.dump`，定期演练 `pg_restore`。
- Redis 无需备份，限流数据丢失不影响业务一致性。
- OCR 单题图片仍只在内存中处理，不落盘；AI 对话附件会保存到 `AI_UPLOAD_DIR`，需与数据库一起备份。
- 备份附件目录：`tar czf ai-uploads-$(date +%F).tar.gz data/ai-uploads`，恢复时保持相同路径与权限（目录 `0750`、文件 `0640`）。
- `AI_UPLOAD_DIR` 建议放在独立数据盘；删除数据库记录不会自动删除文件，需按保留策略定期清理。
- 迁移前必须备份数据库。API 默认在启动时自动迁移，因此发布新版本前应先完成备份；`migrations/*.down.sql` 仅用于受控回滚演练。若需要严格分离，设置 `AUTO_MIGRATE=false` 并在发布流程中显式执行迁移。

## 6. OCR 网络安全与资源

- OCR 仅监听回环或内网地址，安全组/防火墙不得对公网开放 8001。
- 首次识别自动下载模型，预留约 2 GB 磁盘与至少 4 GB 可用内存；生产建议预热模型后再开放录题功能。
- `/healthz` 用于存活检查，`/readyz` 仅在模型加载完成后返回 200。
- Go 客户端已限制 5 MB 图片、真实 MIME、像素上限、并发与超时，上游失败映射为 502/503/504。

## 7. 上线检查清单

1. `APP_ENV=production`、强 `JWT_SECRET`、正确 `CORS_ORIGIN` 已配置。
2. 迁移成功（API 启动日志包含 `database migrations applied`，或手动迁移无报错），`/healthz`、`/readyz` 正常。
3. 首次访问 `/init` 创建学校与管理员，页面仅展示一次学校代码；初始化后重复调用返回 409。
4. 教师注册、录题、管理员审核、组卷、打印全流程人工验收；AI 对话上传 PDF/Word 并导入草稿、刷新后历史与工具调用恢复。
5. 禁用教师或重置密码后旧会话立即失效。
6. 第二所学校越权、CSV 全量回滚、XSS 白名单消毒等自动化测试全部通过。
7. 备份策略生效，日志不含密码、Token、认证头或完整题目图片。
