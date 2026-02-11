## Context

当前 `apps/web` 前端所有数据更新均为硬切换，无任何过渡效果。Dashboard 每 10 秒轮询统计数据，数字直接跳变；日志页通过 WebSocket 实时推送新行，瞬间插入表格；CRUD 操作后列表即时刷新。项目使用 Next.js 16 + Tailwind v4 + TanStack Query v5，尚未引入任何动画库。

## Goals / Non-Goals

**Goals:**

- 为 Dashboard 统计数字添加平滑过渡（number tween）
- 为排名条形图宽度变化添加 CSS 过渡
- 为日志新行添加滑入动画
- 为所有列表 CRUD 操作添加 fade-in/fade-out
- 为 Channel/Group 展开折叠添加高度过渡
- 保持动画在低端设备 60fps

**Non-Goals:**

- 页面级路由转场动画
- 骨架屏（skeleton）加载占位符
- 拖拽动画改进（DnD Kit 已有）
- 图表库（Recharts）内部动画调优

## Decisions

### 1. 动画方案：Motion（framer-motion v12）

**选择**: `motion`（framer-motion v12+ 更名后的包）
**理由**:

- `AnimatePresence` 支持组件卸载动画（列表项删除淡出），CSS 方案无法实现
- `layout` 属性自动处理排序变化的布局动画
- `useSpring` / `useMotionValue` 适合数字平滑过渡
- 与 React 18+ 良好兼容，tree-shakeable（实际使用约 15-20KB gzipped）

**备选方案**:

- ❌ CSS transitions only — 无法处理列表项增删动画和卸载动画
- ❌ react-spring — API 复杂度高，社区活跃度下降
- ❌ auto-animate — 太简单，无法精确控制动画参数

### 2. 数字动画：自定义 AnimatedNumber 组件

**选择**: 基于 `motion` 的 `useSpring` + `useMotionValue` 构建轻量 AnimatedNumber 组件
**理由**:

- 项目中数字格式多样（整数、小数、百分比、带单位），需要灵活的 formatter
- useSpring 自带弹性物理效果，比线性 tween 更自然
- 不需要引入额外的数字动画库

### 3. 列表动画：AnimatePresence + layout

**选择**: 用 `motion.div` 包裹列表项，配合 `AnimatePresence` 处理增删
**理由**:

- 新增：fade + slide-in from top
- 删除：fade-out
- 排序变化：layout animation 自动处理

### 4. 展开/折叠：CSS grid + transition

**选择**: 使用 `grid-template-rows: 0fr → 1fr` + CSS transition
**理由**:

- 纯 CSS 方案，无需 JS 计算高度
- 性能好，浏览器原生优化
- 不需要 motion 库参与

## Risks / Trade-offs

- **[包体积增加]** → motion ~20KB gzipped，可接受。使用 tree-shaking 只引入用到的功能
- **[低端设备性能]** → 所有动画仅使用 transform/opacity（GPU 加速），duration 控制在 200-400ms
- **[动画与数据竞争]** → TanStack Query 10s refetch 可能在动画未完成时触发新数据，使用 `useSpring` 自动中断并过渡到新值
- **[SSR 兼容]** → motion 组件仅在客户端渲染，所有动画页面已是 `"use client"`
