## Why

前端 UI 经过设计团队全面审查，发现了多项交互逻辑缺陷、可访问性缺失、移动端适配不足和错误状态处理遗漏。这些问题影响用户体验的可靠性和易用性，尤其在移动端和键盘/屏读器用户场景下问题突出。需要系统性修复以提升整体质量。

## What Changes

- **替换所有原生 confirm() 为自定义 Dialog** — Settings 和 Prices 页面的删除操作使用原生浏览器对话框，与 neobrutalism 设计不一致
- **添加全局错误状态处理** — Dashboard、Logs 等页面的 useQuery 未处理 error 状态，API 失败时用户无反馈
- **修复移动端触摸交互** — Channels 页面缺少 TouchSensor 导致拖拽失效，多处按钮触摸目标低于 44px 标准
- **改善加载和空状态** — Logs 详情面板仅显示 "Loading..." 文字，Dashboard 统计卡片无 loading 骨架屏，空状态缺少引导
- **添加关键 ARIA 标签** — 主题切换、登出、导航菜单等按钮缺少 aria-label，影响屏读器用户
- **保护 API Key 创建流程** — 新建 key 弹窗可被意外关闭导致 key 永久丢失
- **添加登出确认和认证加载态** — 登出无确认可误触，认证检查中返回 null 导致白屏闪烁
- **修复表单验证和状态管理** — 密码无强度验证、系统配置接受非数字、Dialog 取消后表单未重置

## Capabilities

### New Capabilities

- `confirm-dialog`: 自定义确认对话框组件，替换所有原生 confirm()，支持危险操作警告和后果说明
- `error-empty-states`: 全局错误状态处理和空状态改善，包括查询错误 banner、骨架屏加载态、上下文相关的空状态提示
- `mobile-touch-ux`: 移动端触摸交互修复，包括 TouchSensor 支持、触摸目标尺寸、表格水平滚动提示
- `accessibility-labels`: 全局 ARIA 标签补全，覆盖顶栏按钮、导航、表单和数据可视化组件
- `form-validation-ux`: 表单验证和状态管理改善，包括密码强度、数字输入约束、Dialog 关闭时表单重置
- `auth-flow-polish`: 认证流程优化，包括登出确认、认证加载态、API Key 弹窗保护、登录按钮 spinner

### Modified Capabilities

## Impact

- `apps/web/src/components/app-layout.tsx` — 添加 aria-label、登出确认
- `apps/web/src/app/(protected)/layout.tsx` — 认证加载态
- `apps/web/src/app/login/page.tsx` — 登录 spinner
- `apps/web/src/app/(protected)/dashboard/page.tsx` — 错误/加载/空状态
- `apps/web/src/app/(protected)/channels/page.tsx` — TouchSensor、触摸目标、Dialog 重置、confirm 替换
- `apps/web/src/app/(protected)/logs/page.tsx` — 错误状态、加载骨架屏、空状态区分
- `apps/web/src/app/(protected)/settings/page.tsx` — confirm 替换、API Key 弹窗保护、密码验证、数字输入
- `apps/web/src/app/(protected)/prices/page.tsx` — confirm 替换、表格 overflow
