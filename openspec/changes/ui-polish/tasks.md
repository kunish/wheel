## 1. 替换原生 confirm() 为 AlertDialog

- [x] 1.1 Settings 页面：API key 删除使用 AlertDialog 替代 confirm()，包含 key 名称和后果说明
- [x] 1.2 Settings 页面：channel 相关的 confirm() 调用（如有）替换为 AlertDialog
- [x] 1.3 Prices 页面：价格删除使用 AlertDialog 替代 confirm()，说明对历史成本的影响
- [x] 1.4 Channels 页面：删除 channel 使用 AlertDialog 替代 confirm()
- [x] 1.5 Channels 页面：删除 group 使用 AlertDialog 替代 confirm()
- [x] 1.6 Channels 页面：清空 group items 使用 AlertDialog 替代 confirm()

## 2. 错误状态和加载态

- [x] 2.1 Dashboard：为 stats/chart/heatmap 的 useQuery 添加 error 状态，显示 inline 错误信息和 Retry 按钮
- [x] 2.2 Dashboard：TotalSection 在 data 加载中显示 Skeleton 占位
- [x] 2.3 Dashboard：活动热力图在 today===null 时显示居中 Loader spinner
- [x] 2.4 Logs：为日志列表 useQuery 添加 error 状态处理，显示错误信息和 Retry 按钮
- [x] 2.5 Logs：详情面板加载中显示匹配面板结构的 Skeleton 替代 "Loading..." 文字
- [x] 2.6 Logs：空状态区分"无数据"和"无匹配"，无匹配时显示 "Clear filters" 按钮

## 3. 移动端触摸交互

- [x] 3.1 Channels：添加 TouchSensor（delay: 250, tolerance: 5）支持移动端拖拽
- [x] 3.2 Channels：PointerSensor 激活距离从 5px 增加到 8px 防止与折叠按钮误触
- [x] 3.3 Channels：所有 h-7 w-7 的操作按钮增大到 h-9 w-9（36px 触摸目标）
- [x] 3.4 Channels：ChannelDialog 添加 max-w-[95vw] 和 overflow-y-auto 适配移动端
- [x] 3.5 Logs：表格包裹 overflow-x-auto 容器
- [x] 3.6 Settings：API Keys 表格包裹 overflow-x-auto 容器
- [x] 3.7 Prices：价格表格包裹 overflow-x-auto 容器

## 4. 可访问性标签

- [x] 4.1 app-layout：主题切换按钮添加 aria-label（动态显示当前切换方向）
- [x] 4.2 app-layout：登出按钮添加 aria-label="Logout"
- [x] 4.3 app-layout：移动端菜单按钮添加 aria-label="Open navigation menu"
- [x] 4.4 Settings：API key 复制按钮添加 aria-label="Copy API key <name>"
- [x] 4.5 Prices：编辑按钮添加 aria-label="Edit price for <name>"，删除按钮添加 aria-label="Delete price for <name>"
- [x] 4.6 Prices：搜索框添加 sr-only Label "Search models"

## 5. 表单验证和状态管理

- [x] 5.1 Settings：密码修改添加最小 8 字符验证，不满足时显示错误提示
- [x] 5.2 Settings：系统配置输入框改为 type="number" min="0"
- [x] 5.3 Channels：ChannelDialog onOpenChange 中关闭时重置表单到初始值
- [x] 5.4 Channels：GroupDialog onOpenChange 中关闭时重置表单到初始值
- [x] 5.5 Channels：enable/disable Switch 在 mutation pending 时显示 disabled 状态

## 6. 认证流程优化

- [x] 6.1 app-layout：登出按钮添加 AlertDialog 确认
- [x] 6.2 protected layout：认证检查中显示全屏居中 Loader2 spinner 替代 return null
- [x] 6.3 Settings：API Key 创建弹窗在 createdKey 存在时阻止关闭，显示 "Please copy the key before closing"
- [x] 6.4 Settings：API Key 创建弹窗在用户已复制 key 后允许关闭
- [x] 6.5 login：Sign In 按钮 loading 状态添加 Loader2 spinner 动画
