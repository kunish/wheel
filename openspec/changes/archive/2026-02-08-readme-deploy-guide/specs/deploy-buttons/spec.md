## ADDED Requirements

### Requirement: Vercel 一键部署按钮

README SHALL 包含一个 Vercel Deploy Button，点击后跳转到 Vercel 新建项目页面，预填仓库 URL 和所需环境变量 `NEXT_PUBLIC_API_BASE_URL`。

#### Scenario: 用户点击 Deploy to Vercel 按钮

- **WHEN** 用户在 README 中点击 "Deploy with Vercel" 按钮
- **THEN** 浏览器打开 Vercel 部署页面，仓库 URL 预填为当前项目地址，环境变量 `NEXT_PUBLIC_API_BASE_URL` 显示为必填项

#### Scenario: 按钮图片正常显示

- **WHEN** 用户浏览 README
- **THEN** Deploy to Vercel 按钮 SHALL 使用 Vercel 官方 badge 图片（`vercel.com/button`），在 GitHub 和 npm 页面正常渲染

### Requirement: Cloudflare Workers 一键部署按钮

README SHALL 包含一个 Deploy with Workers 按钮，点击后跳转到 Cloudflare Workers 部署页面。

#### Scenario: 用户点击 Deploy with Workers 按钮

- **WHEN** 用户点击 "Deploy to Cloudflare Workers" 按钮
- **THEN** 浏览器打开 `deploy.workers.cloudflare.com` 页面，自动关联仓库 URL

#### Scenario: 部署后引导设置资源

- **WHEN** 按钮旁显示部署说明
- **THEN** 说明 SHALL 提示用户部署后需要手动创建 D1 Database 和 KV Namespace，并执行数据库迁移
