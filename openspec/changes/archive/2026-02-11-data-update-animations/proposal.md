## Why

当前前端在数据更新时全部采用硬切换（instant swap），Dashboard 统计数字、日志表格、CRUD 操作结果均无任何视觉过渡。这导致频繁轮询（10s）时数字跳变感明显，WebSocket 推送新日志行时突然出现，整体体验缺乏流畅感和专业感。添加细致的动画过渡能显著提升感知质量，同时为用户提供数据变化的视觉线索。

## What Changes

- Dashboard 统计卡片中的数字添加平滑计数动画（number tween/spring）
- Dashboard 排名条形图宽度变化添加平滑过渡
- 日志表格新行从顶部滑入（slide-in），"有新日志"按钮添加脉冲效果
- 所有列表页（channels, groups, apikeys, prices）CRUD 操作后，新建项淡入、删除项淡出
- Channel/Group 卡片展开/折叠添加高度过渡动画
- 通用：数据刷新时 skeleton / shimmer 闪烁提示（而非静默替换）

## Capabilities

### New Capabilities

- `animated-numbers`: 数字平滑过渡组件，用于 Dashboard 统计卡片和排名数据
- `list-transitions`: 列表项增删动画（fade-in/out, slide-in），覆盖日志行、CRUD 结果列表
- `layout-animations`: 展开/折叠、排序变化等布局动画

### Modified Capabilities

（无现有 spec 需要修改）

## Impact

- **依赖**: 需要引入动画库（framer-motion 或类似方案）
- **代码范围**: `apps/web` 内 dashboard, logs, channels, groups, apikeys, prices 页面
- **性能**: 动画需在低端设备上保持 60fps，使用 GPU 加速属性（transform, opacity）
- **包体积**: 动画库增量需控制合理（framer-motion ~30KB gzipped）
