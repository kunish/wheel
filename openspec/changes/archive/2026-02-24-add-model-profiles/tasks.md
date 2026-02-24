## 1. 后端数据层

- [x] 1.1 在 `migrate.go` 中添加 `model_profiles` 表的 DDL（id, name, provider, models, is_builtin, created_at, updated_at）
- [x] 1.2 在 `internal/types` 中添加 `ModelProfile` 结构体
- [x] 1.3 在 `internal/db/dal` 中创建 `profiles.go`，实现 ListProfiles、CreateProfile、UpdateProfile、DeleteProfile、UpsertBuiltinProfile 函数

## 2. 后端 API 层

- [x] 2.1 在 `internal/handler` 中创建 `profile.go`，实现 GET/POST/PUT/DELETE `/api/v1/model/profiles` 端点，包含 builtin 保护逻辑
- [x] 2.2 在路由注册中添加 profile 相关路由
- [x] 2.3 在现有的 model metadata refresh 逻辑中集成 builtin profile 生成：从 models.dev 数据提取 anthropic/openai/google 模型列表，调用 UpsertBuiltinProfile

## 3. 前端 API 与类型

- [x] 3.1 在前端类型定义中添加 `ModelProfile` 接口
- [x] 3.2 在 `api-client.ts` 中添加 profile CRUD 的 API 调用函数
- [x] 3.3 创建 `use-profiles.ts` hook，封装 TanStack Query 的 profile 数据获取与 mutation

## 4. 前端 Profile 管理 UI

- [x] 4.1 创建 `ProfileManageDialog` 组件：展示所有 profile 列表，区分 builtin/custom，支持新建、编辑、删除、复制操作
- [x] 4.2 在模型页面工具栏中添加 "Profiles" 按钮，点击打开 ProfileManageDialog

## 5. 前端 Channel 对话框集成

- [x] 5.1 在 `channel-dialog.tsx` 的模型输入区域上方添加 Profile 选择器下拉组件
- [x] 5.2 实现选择 Profile 后将模型列表合并到当前 channel model 列表的逻辑（去重）

## 6. 国际化

- [x] 6.1 在 en 和 zh-CN 的 i18n 文件中添加 profile 相关翻译字符串
