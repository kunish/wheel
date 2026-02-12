## 1. Docker 配置文件

- [x] 1.1 创建 `apps/web/Dockerfile`：多阶段构建（base → deps → builder → runner），使用 pnpm + standalone 输出
- [x] 1.2 创建 `docker-compose.yml`：Web 服务定义，支持 `.env` 文件和环境变量配置
- [x] 1.3 创建 `.dockerignore`：排除 node_modules、.next、.git 等不必要文件

## 2. README.md 重写

- [x] 2.1 添加顶部 Banner：项目名称、一句话描述、License/Stars badge
- [x] 2.2 添加一键部署按钮区：Vercel Deploy Button + Cloudflare Workers Deploy Button
- [x] 2.3 添加 Features 特性亮点章节
- [x] 2.4 重写部署章节，按渐进顺序排列：一键部署 → Docker 部署 → 手动部署
- [x] 2.5 完善环境变量参考表（增加是否必填、默认值列）
- [x] 2.6 保留开发指南和项目结构章节，微调格式

## 3. 验证

- [x] 3.1 检查所有 Markdown 链接和图片是否正常渲染
- [x] 3.2 验证 Dockerfile 构建流程（`docker build` 能成功执行）
- [x] 3.3 验证 docker-compose.yml 语法正确（`docker compose config`）
