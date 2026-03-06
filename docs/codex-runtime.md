# Codex 渠道配置指引

本文介绍如何在 Wheel 中使用内嵌 Codex 运行时。

## 工作方式

Wheel 会在 worker 进程内自动启动受管的 Codex 运行时，并将数据库中的 Codex 认证文件物化到 Wheel 自己维护的运行时目录。

```text
用户请求 -> Wheel -> embedded Codex runtime -> Codex (OpenAI)
```

你不需要：

- 单独启动额外进程
- 配置本地 auth 目录
- 提供管理密钥
- 手动维护运行时配置文件

## 认证与导入

在 Wheel 管理界面的 Codex 渠道详情中，你可以直接：

- 通过 OAuth 导入账号
- 上传认证文件
- 查看可用模型
- 查看配额
- 同步密钥

Wheel 数据库是认证数据的唯一来源，运行时文件只是自动生成的内部产物。

## 运行时控制

默认情况下，Codex 运行时会自动启动，并在初始化失败或异常退出时直接触发 worker fail-fast。

## 在 Wheel 中创建渠道

1. 进入 **模型 -> 渠道**
2. 创建一个 `Codex` 渠道（`type: 33`）
3. 保存后在渠道详情中使用 OAuth 导入或上传认证文件
4. 同步密钥并获取模型

## 常见问题

### 看不到模型

- 确认 Codex OAuth 已完成或认证文件已上传
- 在渠道详情里点击刷新或重新获取模型

### Worker 启动即退出

- 常见原因是内嵌 Codex 运行时初始化失败
- Wheel 会按 fail-fast 策略直接退出
