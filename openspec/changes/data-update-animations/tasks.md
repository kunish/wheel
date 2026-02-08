## 1. Setup

- [x] 1.1 安装 `motion`（framer-motion v12+）依赖到 `apps/web`
- [x] 1.2 创建 `AnimatedNumber` 组件 (`components/animated-number.tsx`)，基于 useSpring + useMotionValue，支持 formatter 属性

## 2. Dashboard 数字动画

- [x] 2.1 Dashboard TotalSection 4 个统计卡片接入 AnimatedNumber（requests, tokens, cost, time）
- [x] 2.2 Dashboard RankSection 条形图宽度添加 CSS transition（`transition-all duration-300`）
- [x] 2.3 Dashboard ModelStatsSection 使用量条形图宽度添加 CSS transition

## 3. 日志页动画

- [x] 3.1 日志表格新行添加 motion 滑入动画（AnimatePresence + motion.tr）
- [x] 3.2 "X new logs available" 按钮添加 pulse 动画（CSS animate-pulse 或 motion）

## 4. CRUD 列表动画

- [x] 4.1 Channels 页面：创建/删除 channel 时 fade-in/fade-out 动画
- [x] 4.2 Groups 页面：创建/删除 group 时 fade-in/fade-out 动画（与 channels 共页）
- [x] 4.3 API Keys 页面（settings 内）：创建/删除 key 时 fade-in/fade-out 动画
- [x] 4.4 Prices 页面：创建/删除 price 时 fade-in/fade-out 动画

## 5. 展开/折叠 & 布局动画

- [x] 5.1 Channel 卡片展开/折叠添加 CSS grid 高度过渡（grid-template-rows 0fr → 1fr）
- [x] 5.2 Group 卡片展开/折叠添加 CSS grid 高度过渡
- [x] 5.3 Dashboard RankSection 排名变化时使用 motion layout 动画实现平滑重排

## 6. 验证

- [ ] 6.1 验证所有动画在 60fps 下运行，使用 Chrome DevTools Performance 面板
- [x] 6.2 构建确认编译无误
