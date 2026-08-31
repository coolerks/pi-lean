# Progress

## 研究基线

- [x] 读取 `AGENTS.md`、根 `README.md`、根 `package.json`。
- [x] 记录 commit：`853a80d26c90a14c1886f0ebb8ffaae133ca2185`。
- [x] 记录 remote：`https://github.com/coolerks/pi-lean.git`。
- [x] 扫描 `packages/agent/docs`（当前 4 个 Markdown 文件，包含未跟踪的 `harness.v2.md`）。
- [x] 扫描 `packages/coding-agent/docs`（当前 30 个 Markdown 文件）。
- [x] 阅读 Agent Loop、Agent、Agent Harness scaffold、AI types/provider、AgentSession、tools、resource loader、extensions、skills 和 Mintlify 规范。

## 已完成

- [x] 建立独立 `ai-agent-learning/` 目录和当前 Mintlify `docs.json`。
- [x] 建立 21 个原创教材分区、参考资料分区和 Go/Java 示例目录。
- [x] 建立源码索引、术语表、来源说明和中文 Pi 文档映射。
- [x] 原创教材覆盖 LLM、Tool Calling、ReAct、Agent Loop、Context、Session、Compaction、Steering、Harness、Pi 架构、源码、Coding Agent、Extensions、Skills、MCP、RAG、安全和 Telemetry，并为每个重点主题提供实验入口。
- [x] Go 01–15：HTTP/JSON、SSE、Message、Tool、ScriptedModel、Agent Loop、并行、Session、Compaction、Steering、Permission、Skill、MCP、Plugin、Telemetry、RAG Tool 和 Harness；新增 HTTP、MCP、RAG、invalid arguments、multiple calls 测试。
- [x] Java 01–15：HttpClient、SSE、Message、Tool、ScriptedModel、Agent Loop、并行、Session、Compaction、Steering、Permission、Skill、MCP、SPI Plugin、Telemetry 和 Harness；新增 HTTP、MCP、ServiceLoader、invalid arguments 测试。
- [x] Mintlify 初次校验后修复了 MDX angle-bracket 标题和裸 URL；修正 dark color accessibility 对比度。

## 当前阶段

- [x] 对所有原创 MDX 做最终链接、组件、代码块和中文教学内容检查；Mintlify `validate`、`broken-links`、`a11y` 和本地 dev/curl 已执行。
- [x] 对 `pi-docs` 的 34 个源文件完成中文技术译文、章节对照、完整原文附录和来源说明；已提交文件绑定固定 commit，工作区未跟踪的 `harness.v2.md` 单独标注；机器清单与 byte-equivalence 检查通过。
- [x] 将 Go/Java/Mintlify 最终命令输出写入 `VALIDATION.md`，清理构建产物；根 Pi monorepo `npm run check` 的现有 models catalog 类型错误已如实记录。

## 事实边界和已知问题

- `packages/agent/src/harness/agent-harness.ts` 当前是 API scaffold；`AgentHarness.create()` 在发现记录时调用 `HarnessNotImplemented("create.restore")`，多数公共操作返回未实现错误。教材把它作为设计入口和未完成状态说明，不把 `harness.v2.md` 设计稿冒充为已交付运行时。
- 当前 Coding Agent 的可运行主路径是 `Agent` + `AgentSession` + `SessionManager` + `ExtensionRunner`，不是新的 durable harness。
- MCP 不在 `pi-ai` / `pi-agent-core` 的默认核心 provider 列表中；教材把 MCP 作为协议和可接入扩展来讲，不假定 Pi Core 内建 MCP。
- 权限/项目可信度和沙箱能力要以 `packages/coding-agent/docs/security.md`、`src/core/project-trust.ts`、`trust-manager.ts` 及启动配置为准；示例自己的 Permission Layer 只提供教学级路径/命令检查。
- 不执行 Vercel 部署、Git push、PR、线上资源创建，也不写入 API key。
