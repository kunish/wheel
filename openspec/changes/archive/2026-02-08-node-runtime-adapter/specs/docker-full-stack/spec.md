## ADDED Requirements

### Requirement: Worker Dockerfile

项目 SHALL 提供 `apps/worker/Dockerfile`，构建包含 Node.js 运行时的 Worker 生产镜像。

#### Scenario: 构建 Worker 镜像

- **WHEN** 执行 `docker build -f apps/worker/Dockerfile -t wheel-worker .`
- **THEN** SHALL 构建包含编译后代码和 better-sqlite3 原生模块的 Node.js 镜像

#### Scenario: 运行 Worker 容器

- **WHEN** 执行 `docker run -p 8787:8787 -e JWT_SECRET=xxx wheel-worker`
- **THEN** Worker SHALL 在端口 8787 启动，使用容器内 SQLite 文件存储数据

### Requirement: Docker Compose 全栈部署

`docker-compose.yml` SHALL 包含 Web 和 Worker 两个服务，一条命令启动完整系统。

#### Scenario: 一键启动全栈

- **WHEN** 用户配置 `.env` 后执行 `docker compose up -d`
- **THEN** Web 仪表盘（:3000）和 Worker API（:8787）SHALL 同时启动，Web 自动连接 Worker

#### Scenario: 数据持久化

- **WHEN** docker-compose.yml 定义 volume 挂载
- **THEN** Worker 的 SQLite 数据库文件 SHALL 持久化到宿主机 `./data/` 目录

### Requirement: .env.example 配置模板

项目 SHALL 提供 `.env.example` 文件，列出 Docker 部署所需的全部环境变量及说明。

#### Scenario: 用户配置环境变量

- **WHEN** 用户复制 `.env.example` 为 `.env`
- **THEN** 文件 SHALL 包含 `JWT_SECRET`、`ADMIN_USERNAME`、`ADMIN_PASSWORD` 等必填变量及其默认值说明
