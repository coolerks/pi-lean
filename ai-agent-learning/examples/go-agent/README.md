# Go Agent 示例

Go 1.21+、仅标准库。代码入口在 `internal/agent`，CLI 在 `cmd/agent`。默认模型是离线 `ScriptedModel`；HTTP 模式使用 OpenAI-compatible Chat Completions，并且只从 `AGENT_API_KEY` 读取密钥。

```bash
go test ./...
go vet ./...
go run ./cmd/agent --demo
```

阶段与源码：

| 阶段 | 文件/能力 |
| --- | --- |
| Go 01 | `model.go`：HTTP JSON `Model` |
| Go 02 | `model.go`：SSE `StreamingModel` |
| Go 03 | `tool.go`：Tool Definition、JSON Schema gate |
| Go 04 | `loop.go`：有限 Agent Loop、Tool Result 回传 |
| Go 05 | `builtin_tools.go`：read/write/edit/grep/shell |
| Go 06 | `model.go`：Message、Request、Provider payload |
| Go 07 | `loop.go`：retry、backoff、`context.Context` abort |
| Go 08 | `loop.go`：events、Telemetry recorder |
| Go 09 | `session.go`：JSONL 保存与恢复 |
| Go 10 | `session.go`：token estimate、compaction |
| Go 11 | `agent.go`：steering/follow-up/next-run queues |
| Go 12 | `plugin.go`：interface + explicit registry |
| Go 13 | `skill.go`：SKILL.md metadata discovery |
| Go 14 | `mcp.go`：JSON-RPC stdio client |
| Go 15 | `harness.go`、`telemetry.go`：组合层 |
| RAG 实验 | `rag.go`：tenant filter + cited term retrieval Tool |

每一层都有离线测试；`ScriptedModel` 第一轮可返回 Tool Call，第二轮返回 final。`PermissionPolicy` 和 MCP client 是教学实现，不是生产 sandbox；`07-harness/reliability` 说明 durable operation log、崩溃恢复和副作用幂等还需要怎样补全。
