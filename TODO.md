# codeg - Coding Agent

基于 Go 的 coding agent，结合 pi 的 coding agent 能力和 beads 的上下文管理能力。

## 项目概述

- **目标**：创建一个具备结构化任务管理能力的 coding agent
- **核心特性**：
  - 交互式 CLI
  - LLM 对话与工具调用
  - beads 风格的任务/依赖管理
  - 会话持久化
  - 上下文压缩

## 模块 TODO

### 1. cmd/codeg

主入口点

- [ ] 实现 CLI 参数解析
- [ ] 支持交互模式 (Interactive Mode)
- [ ] 支持打印模式 (Print Mode)
- [ ] 支持 RPC 模式
- [ ] 处理 stdin/stdout
- [ ] 优雅退出与信号处理

### 2. internal/agent

Agent 核心逻辑

#### internal/agent/core

- [ ] `Agent` 结构体定义
- [ ] `Config` 配置结构
- [ ] `New()` 工厂方法
- [ ] `Run()` 主循环
- [ ] `Step()` 单步执行
- [ ] `HandleToolCall()` 工具调用处理
- [ ] `BuildContext()` 上下文构建
- [ ] 事件系统 (start/end/tool_call/error)

#### internal/agent/loop

- [ ] 对话循环逻辑
- [ ] 思考过程 (thinking) 处理
- [ ] Token 预算管理
- [ ] 最大轮次控制
- [ ] 错误处理与重试

#### internal/agent/prompt

- [ ] System Prompt 模板
- [ ] 工具描述生成
- [ ] 上下文格式化
- [ ] 会话历史格式化

### 3. internal/llm

LLM 接口层

#### internal/llm/client

- [ ] `Client` 接口定义
- [ ] `Message` 消息结构 (role, content, tool_calls)
- [ ] `Request` 请求结构
- [ ] `Response` 响应结构
- [ ] `Stream()` 流式响应

#### internal/llm/providers

##### openai

- [ ] OpenAI API 客户端
- [ ] GPT-4o / GPT-4o-mini 支持
- [ ] 流式响应支持
- [ ] Tool calling 支持

##### anthropic

- [ ] Anthropic API 客户端
- [ ] Claude 3.5/3 支持
- [ ] 流式响应支持
- [ ] Tool calling 支持
- [ ] Thinking/Reasoning 支持

##### google

- [ ] Google AI API 客户端
- [ ] Gemini 支持

#### internal/llm/types

- [ ] 通用类型定义
- [ ] Token 计算
- [ ] 模型能力检测

### 4. internal/tools

代码工具集

#### internal/tools/core

- [ ] `Tool` 接口定义
- [ ] `ToolDefinition` 工具描述 (name, description, input_schema)
- [ ] `ToolResult` 工具结果
- [ ] 工具注册表

#### internal/tools/impl

##### read

- [ ] 读取文件内容
- [ ] 行号范围支持
- [ ] 大文件截断
- [ ] 二进制文件检测

##### write

- [ ] 写入文件
- [ ] 创建新文件
- [ ] 覆盖确认

##### edit

- [ ] 精确行编辑
- [ ] 多处编辑支持
- [ ] 编辑验证

##### bash

- [ ] 命令执行
- [ ] 工作目录支持
- [ ] 超时控制
- [ ] 环境变量
- [ ] 输出捕获

##### grep

- [ ] 正则搜索
- [ ] 文件过滤
- [ ] 行号返回
- [ ] 上下文行

##### find

- [ ] 文件查找
- [ ] glob 模式
- [ ] 目录过滤
- [ ] 类型过滤

##### ls

- [ ] 目录列表
- [ ] 详细信息
- [ ] 隐藏文件

#### internal/tools/registry

- [ ] 内置工具注册
- [ ] 工具发现机制
- [ ] 工具分类

### 4.1 internal/skills

技能系统 (借鉴 pi 的 Agent Skills)

#### internal/skills/types

- [ ] `Skill` 结构体 (name, description, filePath, baseDir, source)
- [ ] `SkillFrontmatter` (name, description, disable-model-invocation)
- [ ] `LoadSkillsResult` 加载结果
- [ ] `SkillSource` 来源类型 (user, project, path)

#### internal/skills/loader

- [ ] `LoadSkills()` 从目录加载技能
- [ ] `LoadSkillsFromDir()` 从指定目录加载
- [ ] `LoadSkillFromFile()` 从文件加载单个技能
- [ ] 验证技能名称 (小写字母、数字、连字符)
- [ ] 验证描述长度
- [ ] 支持 `SKILL.md` 命名规范
- [ ] 支持根目录 `.md` 文件作为技能

#### internal/skills/prompt

- [ ] `FormatSkillsForPrompt()` 格式化为 XML
- [ ] 支持 `disable-model-invocation` 属性
- [ ] 相对路径解析规则
- [ ] Skill 调用指令生成

#### internal/skills/discovery

- [ ] 默认技能目录发现
- [ ] 用户技能目录 (`~/.codeg/skills`)
- [ ] 项目技能目录 (`{project}/.codeg/skills`)
- [ ] 显式技能路径支持 (`--skill` 参数)
- [ ] 忽略规则 (`.gitignore`, `.ignore`)
- [ ] 符号链接处理
- [ ] 技能名称冲突检测

#### internal/skills/command

- [ ] `/skill:<name>` 命令解析
- [ ] 技能执行上下文
- [ ] 技能文件内容读取
- [ ] 相对路径转换

### 5. internal/beads

任务/依赖管理 (基于 beads 设计)

#### internal/beads/storage

- [ ] `Storage` 接口
- [ ] `Transaction` 事务
- [ ] Dolt 后端实现 (可选，可简化使用 SQLite)

#### internal/beads/types

- [ ] `Issue` 任务结构 (id, title, description, status, type, priority)
- [ ] `Status` 状态枚举 (open, in_progress, blocked, closed)
- [ ] `IssueType` 类型 (bug, feature, task, epic)
- [ ] `Dependency` 依赖结构
- [ ] `DependencyType` 依赖类型 (blocks, related, parent_child)
- [ ] `Event` 事件记录
- [ ] `Label` 标签

#### internal/beads/repository

- [ ] `IssueRepository` Issue 增删改查
- [ ] `DependencyRepository` 依赖管理
- [ ] `EventRepository` 事件记录

#### internal/beads/service

- [ ] `TaskService` 任务服务
- [ ] `DependencyService` 依赖服务
- [ ] `ReadyDetector` 就绪任务检测 (无阻塞任务)
- [ ] `GraphBuilder` 依赖图构建

### 6. internal/session

会话管理

#### internal/session/manager

- [ ] `SessionManager` 会话管理器
- [ ] `Create()` 创建新会话
- [ ] `Open()` 打开会话
- [ ] `Save()` 保存会话
- [ ] `List()` 列出会话
- [ ] `Continue()` 继续最近会话
- [ ] `Fork()` 分叉会话

#### internal/session/types

- [ ] `Session` 会话结构
- [ ] `Message` 消息条目
- [ ] `SessionEntry` 条目类型 (user, assistant, tool_call, tool_result)
- [ ] `SessionHeader` 头部信息

#### internal/session/store

- [ ] `Store` 存储接口
- [ ] JSON 文件存储实现
- [ ] 会话迁移机制

### 7. internal/compaction

上下文压缩 (借鉴 pi)

#### internal/compaction/core

- [ ] `Compactor` 压缩器接口
- [ ] `Settings` 压缩设置
- [ ] `ShouldCompact()` 判断是否需要压缩
- [ ] `Compact()` 执行压缩

#### internal/compaction/strategy

- [ ] `TokenBudget` Token 预算策略
- [ ] `LastNStrategy` 保留最近 N 条
- [ ] `SummaryStrategy` 摘要策略
- [ ] `CutPointFinder` 切割点查找

#### internal/compaction/summary

- [ ] `BranchSummary` 分支摘要
- [ ] `GenerateSummary()` 生成摘要
- [ ] `SummarizeOldEntries()` 旧条目摘要

### 8. internal/cli

交互式 CLI

#### internal/cli/ui

- [ ] `UI` 界面接口
- [ ] 终端检测
- [ ] 颜色支持
- [ ] 进度显示

#### internal/cli/components

- [ ] 消息渲染 (用户/助手/工具)
- [ ] 工具执行显示
- [ ] 确认对话框
- [ ] 主题支持

#### internal/cli/input

- [ ] 用户输入处理
- [ ] 快捷键支持
- [ ] 自动补全

### 9. internal/config

配置管理

- [ ] `Config` 全局配置
- [ ] `Load()` 加载配置
- [ ] `Save()` 保存配置
- [ ] `API Key` 管理
- [ ] `Model` 模型配置

## 外部依赖

- `github.com/spf13/cobra` - CLI 框架
- `github.com/sashabaranov/go-openai` - OpenAI SDK
- `github.com/anthropic-ai/sdk-go` - Anthropic SDK
- `github.com/google/generative-ai-go` - Google AI SDK
- `github.com/dolthub/go-sdk` - Dolt SDK (可选)
- `github.com/mattn/go-isatty` - 终端检测
- `github.com/munnerz/goautoneg` - 优先级排序

## 里程碑

### Phase 1: 基础框架
- [ ] 项目初始化
- [ ] 基础 CLI
- [ ] LLM 客户端 (单 Provider)
- [ ] 基础工具集 (read/write/bash)

### Phase 2: Agent 核心
- [ ] Agent 主循环
- [ ] 工具调用
- [ ] Session 管理
- [ ] 上下文构建

### Phase 3: 任务管理
- [ ] Issue/Dependency 类型
- [ ] 任务 CRUD
- [ ] 依赖图
- [ ] 就绪检测

### Phase 4: 技能系统
- [ ] Skill 类型定义
- [ ] Skill 加载器 (user/project/path)
- [ ] Skill 验证 (名称、描述)
- [ ] Skill Prompt 格式化
- [ ] `/skill:<name>` 命令支持

### Phase 5: 完善
- [ ] 上下文压缩
- [ ] 多 Provider 支持
- [ ] 交互式 UI
- [ ] 测试

## 设计原则

1. **模块化**：每个模块独立，可单独测试
2. **接口驱动**：依赖接口而非具体实现
3. **配置优先**：通过配置控制行为
4. **错误友好**：清晰的错误信息与恢复机制
5. **可扩展**：易于添加新工具和新 Provider
