# codeg v2 — OpenAI & ACP Call Point Review

## 1. OpenAI API Call Chain

### 1.1 Architecture

```
main.go
  └─ llm.NewOpenAIClient(config)          ← 创建 OpenAI 客户端
       └─ TaskManager                      ← 持有 llm.Client
            ├─ Planner                      ← 计划模式分解任务
            │    └─ DecomposeTask()
            │         └─ p.client.Chat() → POST /v1/chat/completions   [调用点 ①]
            │
            └─ GenerateSummary()            ← 任务完成后生成摘要
                 └─ m.llm.Summarize() → POST /v1/chat/completions      [调用点 ②]
```

### 1.2 调用点详解

**调用点 ① — 计划模式（Plan Mode）**
| 项目 | 内容 |
|------|------|
| 触发 | 用户在 TUI 中按 `p` 键 |
| 入口 | `app.go:planTaskCmd()` → `Planner.DecomposeTask()` |
| 端点 | `{baseURL}/v1/chat/completions` |
| 模型 | 全局配置的模型（如 `deepseek-chat`） |
| 系统提示 | 项目规划器角色定义 |
| 用户消息 | 任务标题 + 目标 + 可用目录列表 |
| 输出 | JSON 子任务数组 `[{title, description, directory}]` |

**调用点 ② — 任务摘要（Task Summary）**
| 项目 | 内容 |
|------|------|
| 触发 | 所有子任务完成后自动触发 |
| 入口 | `TaskManager.CompleteTask()` → goroutine → `GenerateSummary()` |
| 端点 | `{baseURL}/v1/chat/completions` |
| 模型 | 全局配置的模型 |
| 系统提示 | 技术摘要器角色定义 |
| 用户消息 | 目标 + 会话历史 |
| 输出 | 文本摘要 |
| 注意 | **异步执行，使用 context.Background()，应用退出时 goroutine 可能泄漏** |

### 1.3 配置

```bash
OPENAI_API_KEY="sk-xxx"              # API 密钥
OPENAI_BASE_URL="https://api.deepseek.com"  # 基础 URL
CODEG_LLM_MODEL="deepseek-chat"      # 模型名
CODEG_PLANNING_MODEL="deepseek-chat" # 计划专用模型（可选，未使用）
```

---

## 2. ACP 调用链

### 2.1 架构

```
main.go
  ├─ [托管模式] AcpManager.StartServer()
  │    └─ exec "opencode serve --port 0"       ← 启动子进程
  │    └─ HealthCheck() → Ping()                ← 等待就绪
  │    └─ Initialize()                            ← ACP 握手
  │
  ├─ [外部模式] 直接使用 CODEG_ACP_URL
  │
  └─ opencode.NewRunner(serverURL)               ← 创建 Runner
       └─ AgentController                         ← 持有 Runner
            ├─ StartSubTaskCoding()               ← 编码阶段
            │    └─ Runner.Start(cwd, prompt)
            │         ├─ Initialize()               ← ACP 握手（幂等）
            │         ├─ CreateSession()            ← POST session/new   [①]
            │         ├─ SendPrompt()               ← POST session/prompt [②]
            │         └─ SubscribeEvents()          ← GET ?sessionId=x (SSE) [③]
            │
            └─ StartSubTaskVerification()          ← 验证阶段
                 └─ Runner.Start(cwd, prompt)     ← 同上流程
```

### 2.2 ACP 调用序列

每次启动一个 agent 阶段（编码或验证），执行以下 JSON-RPC 调用：

```
1. POST /api/acp  {"method":"initialize", "params":{"protocolVersion":1}}
   ← {"protocolVersion":1, "serverName":"..."}

2. POST /api/acp  {"method":"session/new", "params":{"sessionId":"...","workspaceId":"codeg","role":"DEVELOPER","cwd":"/path"}}
   ← {"sessionId":"ses_xxx", "status":"active"}

3. POST /api/acp  {"method":"session/prompt", "params":{"sessionId":"ses_xxx","prompt":"Implement..."}}
   ← {"sessionId":"ses_xxx", "turnId":"turn_1", "status":"streaming"}

4. GET /api/acp?sessionId=ses_xxx  (SSE 长连接)
   ← event: agent_message_chunk → data: {"text":"..."}
   ← event: tool_call           → data: {"name":"read","args":{...}}
   ← event: tool_call_update    → data: {"output":"..."}
   ← event: turn_complete       → data: {}

5. [取消] POST /api/acp  {"method":"session/cancel", "params":{"sessionId":"ses_xxx"}}
```

### 2.3 关键发现

**发现 #1：Role 硬编码为 DEVELOPER**

`opencode.Runner.Start()` 总是传递 `Role: "DEVELOPER"`：
```go
Role: string(acp.RoleDeveloper),  // ← 硬编码
```
验证阶段的 agent 也应该以 DEVELOPER 运行，但提示词不同（通过 `buildVerificationPrompt` 生成）。这实际上是可以的——GATE 角色只是一个语义标记，实际的"验证"行为由提示词控制。

**发现 #2: AcpManager.CreateSession() 是死代码**

`AcpManager.CreateSession()` → `AcpSession.Start()` 实现了完整的 session 创建+提示+SSE 流，但**从未被调用**。实际的 agent 执行通过 `AgentController` → `openCode.Runner.Start()` 走。

**发现 #3: 重复的 Initialize 调用**

- `AcpManager.StartServer()` 调用 `m.client.Initialize(ctx)`（在 AcpManager 自己的 client 上）
- `openCode.Runner.Start()` 也调用 `r.client.Initialize(ctx)`（在 Runner 自己的 client 上）

当使用托管模式时，Runner 通过 `AcpManager.ServerURL()` 创建自己的 client（另一个 `ACPClient` 实例）。由于 idempotency，重复调用不会出错，但每个 Runner 的 client 实例是独立的。

**发现 #4: SSE 流处理路径**

TUI 的事件流：
```
ACP Server SSE → AcpClient.parseSSE() → channel → streamAgentEvents()
  → m.agentEvents channel → waitForAgentEvent() → Update()
    → agentPanel.Update() → View 刷新
```

### 2.4 ACP Server 配置

```bash
CODEG_ACP_URL="http://localhost:4200/api/acp"   # 外部 ACP 服务器
CODEG_ACP_COMMAND="opencode serve"               # 托管模式的启动命令
CODEG_ACP_PORT_FLAG="--port"                     # 端口参数前缀
CODEG_ACP_PORT="0"                               # 端口号（0=随机）
CODEG_ACP_PROVIDER="anthropic"                   # 提供商
```

---

## 3. 问题与建议

| # | 严重度 | 问题 | 建议 |
|---|--------|------|------|
| 1 | **中** | `AcpManager.CreateSession()` + `AcpSession.Start()` 是死代码，从未被调用 | 移除或重构为单一 session 创建路径 |
| 2 | **低** | `GenerateSummary()` goroutine 使用 `context.Background()`，应用退出时可能泄漏 | 使用带 cancel 的 context |
| 3 | **低** | `Planner.model` 字段设置但未使用，无法为计划模式指定不同的模型 | 实现 `Planning.Model` 配置的实际使用 |
| 4 | **低** | `openCode.Runner.Start()` 总是传递 `RoleDeveloper`，验证阶段也一样 | 通过参数传递正确的 role |
| 5 | **低** | 托管模式下 Runner 创建新的 ACPClient 而非复用 AcpManager 的 client | 可考虑共享 client 实例 |

---

## 4. 数据流总结

```
用户操作 (TUI)
  │
  ├─ 创建任务 ──────────────────────────────► SQLite 存储
  │
  ├─ 绑定目录 ──────────────────────────────► SQLite 存储
  │
  ├─ 计划模式 (p) ──► OpenAI API ──────────► 子任务列表 → SQLite
  │
  ├─ 运行编码 (r) ──► ACP session/new ─────► ACP session/prompt
  │                   │                        │
  │                   │                   opencode agent 执行
  │                   │                        │
  │                   │                   ACP SSE 事件流
  │                   │                        │
  │                   │                   TUI agent_panel 实时显示
  │                   │                        │
  │                   └─ turn_complete ───────► 自动启动验证阶段
  │
  ├─ 验证阶段 (自动) ─► 同上 ACP 流程 ──────► 完成 / 失败
  │
  └─ 全部完成 ────────► OpenAI API ──────────► 生成摘要 → SQLite
```
