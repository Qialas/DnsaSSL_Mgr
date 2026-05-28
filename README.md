# DaSSLm

> 开源的一站式 DNS 与 SSL 证书管理系统。  
> Dns and SSL Manager.

![DaSSLm Preview](https://assets.oss.qialas.com/images/udd1f25b948024046/460ec753bcd340eda675ee81accd4b0d.png)

DaSSLm 面向需要统一管理域名解析、SSL 证书申请、证书资产和自动化部署的个人开发者与小团队。项目采用 Go + MySQL + React 技术栈，管理后台可被嵌入 Go 服务端。

## 功能特性

- 多厂商 DNS 账号管理：支持阿里云 DNS、DNSPod / 腾讯云 DNS、Cloudflare 等服务接入。
- 域名资产同步：从 DNS 服务商拉取域名，保存解析商、解析记录数、域名到期时间等信息。
- DNS 解析记录管理：支持解析记录列表、线路查询、新增、编辑、删除与同步。
- SSL 账号管理：支持 Let's Encrypt、ZeroSSL、自定义 ACME、腾讯云免费证书、阿里云免费证书等账号类型配置。
- 证书资产管理：独立证书页面，支持证书申请记录、证书状态、到期时间、续签提前天数等管理。
- 腾讯云 SSL 对接：支持腾讯云免费证书申请、DNS 验证记录补全、证书资源拉取与本地保存。
- 自动化与日志：预留自动任务管理，记录登录、DNS、证书等关键操作日志。
- 单程序部署：前端构建产物可嵌入 Go 二进制，部署时无需单独维护前端运行时。

## 技术栈

| 模块 | 技术 |
| --- | --- |
| 服务端 | Go、Gin、GORM、MySQL、JWT |
| 管理端 | React、Vite、Ant Design、Ant Design Pro Components |
| DNS 接入 | Alibaba Cloud DNS、Tencent Cloud DNSPod、Cloudflare API |
| SSL 接入 | ACME 账号模型、Tencent Cloud SSL |
| 部署 | Linux amd64 打包、静态资源嵌入、单二进制运行 |

## 项目结构

```text
.
├── admin-web/          # React 管理后台
└── server/             # Go 服务端、API、数据库模型与打包脚本
```

## 快速开始

### 1. 准备数据库

先创建 MySQL 数据库，默认库名为 `qdl`。服务端启动时会自动执行表结构迁移，数据库本身需要提前存在。

```sql
CREATE DATABASE qdl DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 2. 配置服务端

```bash
cd server
cp config/config.example.yaml config/config.yaml
```

编辑 `server/config/config.yaml`，填写 MySQL 连接信息，并在生产环境修改 `jwt.secret`。

### 3. 启动服务端

```bash
cd server
go mod tidy
go run ./cmd/qdl-server
```

默认监听地址：

```text
http://localhost:8080
```

首次启动时，如果 `server/config/install.lock` 不存在，系统会创建默认管理员并写入安装锁。

```text
用户名：admin
密码：123456
```

生产环境请在首次登录后立即修改默认密码。

### 4. 启动管理端开发环境

```bash
cd admin-web
npm install
npm run dev
```

管理端默认 API 地址为 `http://localhost:8080/api`，可通过 `VITE_API_BASE` 覆盖。生产构建中推荐使用相对路径 `/api`。

## Linux 打包部署

项目提供一键 Linux amd64 打包脚本。脚本会先构建 `admin-web`，再同步到 `server/internal/web/dist` 并嵌入 Go 二进制。

```bash
cd server
chmod +x scripts/package-linux.sh
./scripts/package-linux.sh
```

打包产物：

```text
server/dist/qdl-server-linux-amd64.tar.gz
```

上传到服务器后解压，修改 `config/config.yaml`，然后运行：

```bash
./qdl-server
```

运行后可直接访问服务端端口打开管理后台：

```text
http://服务器IP:8080/
```

## 常用命令

```bash
# 服务端编译检查
cd server && go build ./cmd/qdl-server

# 服务端测试
cd server && go test ./...

# 管理端构建
cd admin-web && npm run build

# Linux 发布包
cd server && ./scripts/package-linux.sh
```

## 配置说明

服务端配置文件位于 `server/config/config.yaml`：

```yaml
server:
  host: 0.0.0.0
  port: 8080
  mode: debug

database:
  host: 127.0.0.1
  port: 3306
  username: root
  password: ""
  database: qdl
  charset: utf8mb4
  parseTime: true
  loc: Local

jwt:
  secret: "please-change-this-secret"
  expireHours: 24
```

