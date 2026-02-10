## Context

Wheel 前端使用 Next.js + Tailwind v4 + shadcn/ui，采用 neobrutalism 设计语言。经过全面 UX 审查，发现 146 个问题（8 Critical、20 High、62 Medium、56 Low）。本轮聚焦修复 Critical 和 High 级别问题，以及影响面广的 Medium 问题。

当前主要痛点：

- 危险操作使用原生 `confirm()` 对话框，与设计系统脱节
- 多个页面的 useQuery 缺少 error/loading 处理，API 失败时无反馈
- 移动端拖拽失效（缺少 TouchSensor）、触摸目标过小、表格无水平滚动提示
- 屏读器用户无法使用多个功能（缺少 aria-label）
- 认证流程存在白屏闪烁、误触登出、API Key 意外丢失等问题

## Goals / Non-Goals

**Goals:**

- 替换所有原生 confirm() 为 shadcn AlertDialog 组件
- 为 Dashboard 和 Logs 添加 query error 状态处理
- 修复 Channels 移动端拖拽和触摸交互
- 补全关键 ARIA 标签（顶栏、导航、表单）
- 保护 API Key 创建弹窗防止意外关闭
- 添加登出确认、认证加载态、登录 spinner
- 修复密码验证、数字输入约束、Dialog 状态重置

**Non-Goals:**

- 不添加新功能（如命令面板、快捷键系统、虚拟滚动）
- 不重构组件架构或创建新的抽象层
- 不修改后端 API
- 不处理 Low 级别的视觉微调（如动画时长、渐变方向等）

## Decisions

1. **使用 shadcn AlertDialog 替换 confirm()** — 项目已安装 shadcn/ui，AlertDialog 提供原生模态行为（焦点陷阱、ESC 关闭）且与设计系统一致。不创建自定义组件，直接在使用处内联 AlertDialog。理由：只有 3-4 处使用，不值得抽象。

2. **错误状态使用 inline error banner** — 在每个页面的数据区域内显示错误信息和重试按钮，而非全局 toast。理由：toast 容易被忽略且会自动消失，inline banner 持久可见且与失败的数据区域关联。

3. **TouchSensor 使用 delay 激活** — 设置 `delay: 250, tolerance: 5` 防止滚动时误触发拖拽。理由：与 iOS/Android 原生长按拖拽行为一致。

4. **API Key 弹窗保护使用 onOpenChange 拦截** — 当 createdKey 存在时阻止关闭，强制用户先复制 key。理由：比隐藏关闭按钮更安全（也阻止了 ESC 和背景点击）。

5. **认证加载态使用全屏 spinner** — 替换当前的 `return null`，显示居中的 Loader2 spinner。理由：简单直接，比骨架屏更合适（认证检查通常极快）。

## Risks / Trade-offs

- [AlertDialog 内联使用] → 如果未来更多地方需要确认，会产生重复代码。但当前只有 3-4 处，过早抽象不划算。
- [TouchSensor delay] → 250ms 延迟可能让移动端拖拽感觉略迟钝。但这是误触发和响应性之间的合理平衡。
- [阻止 API Key 弹窗关闭] → 用户可能困惑为什么弹窗关不掉。需要添加明确的提示文字说明。
