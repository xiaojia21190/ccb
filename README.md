# CCB - Claude Code Bridge

多模型 AI 协作桥接工具，支持 Claude、Codex、Gemini 在终端中并行运行。

## 功能

- 在 WezTerm 中启动多个 AI CLI 会话
- 异步/同步发送消息到各模型
- 查看模型历史输出
- 自动管理会话状态

## 安装

```bash
go build -o ccb .
```

## 使用

### 启动服务

```bash
# 启动所有模型（布局：左 Claude，右上 Gemini，右下 Codex）
ccb up claude codex gemini

# 单独启动
ccb up codex
ccb up gemini
```

### 发送消息

```bash
# 异步发送
ccb cask "分析这段代码"    # Codex
ccb gask "设计 UI 原型"    # Gemini

# 同步等待响应
ccb cask-w "问题"
ccb gask-w --timeout 120 "问题"
```

### 查看输出

```bash
ccb cpend 5    # Codex 最近 5 条
ccb gpend 1    # Gemini 最近 1 条
```

### 其他命令

```bash
ccb status     # 查看状态
ccb kill       # 停止所有
ccb cping      # 测试 Codex 连接
ccb gping      # 测试 Gemini 连接
```

## 环境要求

- Go 1.20+
- WezTerm
- Claude CLI / Codex CLI / Gemini CLI

## 许可

MIT
