## ADDED Requirements

### Requirement: 组件导出方式统一

`components/` 目录下的组件 SHALL 统一使用命名导出 `export function`。`pages/` 目录下的页面组件 SHALL 统一使用 `export default function`。

#### Scenario: components 目录使用命名导出

- **WHEN** `components/chart-section.tsx` 导出组件
- **THEN** SHALL 使用 `export function ChartSection`，不使用 `export default`

#### Scenario: pages 目录使用默认导出

- **WHEN** 页面组件需要导出
- **THEN** SHALL 使用 `export default function XxxPage`，配合路由 lazy loading

### Requirement: Props 类型定义统一为命名 interface

组件的 props 类型 SHALL 统一使用命名 `interface XxxProps` 定义。仅当 props 只有 1 个属性时可以使用内联类型。

#### Scenario: 多属性 props 使用命名 interface

- **WHEN** 组件有 2 个或以上 props
- **THEN** SHALL 定义 `interface XxxProps` 并在函数签名中引用

#### Scenario: 单属性 props 可内联

- **WHEN** 组件仅有 1 个 prop
- **THEN** 可以使用内联类型 `({ value }: { value: string })`

### Requirement: 纯类型导入使用 import type

所有仅用于类型注解的导入 SHALL 使用 `import type` 语法。

#### Scenario: 类型导入使用 import type

- **WHEN** 从模块导入仅用于类型注解的标识符
- **THEN** SHALL 使用 `import type { Xxx } from '...'` 语法
