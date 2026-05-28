# QDL 服务端

QDL 是一个集 DNS 管理、SSL 证书申请、SSL 自动化申请/部署于一体的系统。本服务端使用 Go + Gin + GORM + MySQL。

## 配置

数据库配置放在 `config/config.yaml`。服务端启动时会自动执行表结构迁移并创建缺失的数据表，数据库本身需要提前存在。

首次启动时，如果 `config/install.lock` 不存在，服务端会创建初始管理员并写入安装锁；安装锁存在后，后续启动不会再创建或更新管理员账号。

- 用户名：`admin`
- 密码：`123456`

生产环境请务必修改 `jwt.secret`，登录后及时修改默认管理员密码。

## 本地运行

```bash
go mod tidy
go run ./cmd/qdl-server
```

健康检查：

```bash
curl http://localhost:8080/health
```

登录接口：

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"123456"}'
```

除 `/health` 和 `/api/auth/login` 外，其他接口都需要携带：

```text
Authorization: Bearer <token>
```

## Linux 打包

每次更新服务端后运行：

```bash
chmod +x scripts/package-linux.sh
./scripts/package-linux.sh
```

产物：

```text
dist/qdl-server-linux-amd64.tar.gz
```

打包脚本会先构建 `../admin-web` 并同步到 `internal/web/dist`，再把管理端静态文件嵌入 Go 二进制。部署后只运行 `qdl-server` 即可同时提供管理端页面和 `/api` 接口。
