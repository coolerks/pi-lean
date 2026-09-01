# AI Agent 工程教材

这是一个独立的 Mintlify 文档站，使用当前工作区的 Pi 源码作为案例，并提供 Go、Java 的离线可测试 Agent 实现。它不修改 Pi 原有英文文档或核心业务源码；生产发布通过关联的 GitHub 仓库触发 Vercel 自动部署。

## 阅读方式

从 `index.mdx` 开始，按 `00-guide` 到 `20-frameworks` 阅读。`pi-docs/` 是 Pi 官方文档的中文参考版，和前面的原创学习教材分开；`reference/` 保存术语表、源码索引和来源。

推荐先完成：

1. `01-foundations` 到 `04-tools`：模型、消息、Tool Calling、ReAct 和循环。
2. `05-context` 到 `07-harness`：Context、Session、Compaction、Steering 与可靠性。
3. `08-pi-architecture` 到 `10-coding-agent`：从仓库结构进入源码调用链。
4. `17-go-agent` 与 `18-java-agent`：运行每个阶段的测试。
5. `19-build-harness`、`20-frameworks`：做架构取舍和框架反向验证。

## 本地启动

在本目录使用 Mintlify CLI 运行：

```bash
cd ai-agent-learning
npx --yes mint@latest validate
npx --yes mint@latest broken-links
npx --yes mint@latest dev
```

`mint dev` 默认提供本地预览。生产部署通过 GitHub 的 `main` 分支触发，不要从本地直接执行 Vercel CLI 部署，以免绕过仓库集成。

## Vercel 部署

当前 Vercel 项目为 `pi-agent-learning`，关联 GitHub 仓库 `coolerks/pi-lean`，项目根目录为 `ai-agent-learning`。向 `main` 推送后，Vercel 会自动构建并发布文档站。

Vercel 项目设置中的 `Vercel Toolbar → Production` 已关闭。这个开关属于 Vercel 项目设置，不是 Mintlify `docs.json` 或受支持的 `vercel.json` 配置项，因此不要在仓库中添加同名字段伪造配置。

黑色圆形按钮实际是 `mint export` 客户端中的 Mintlify Hot Reloader 状态组件，不是 Vercel Toolbar。`style.css` 用稳定的 `role="status"`、`fixed` 和 `z-999` 选择器将它从生产页面隐藏；本地 `mint dev` 仍可显示自己的开发状态。

## 运行 Go 示例

需要 Go 1.25 或兼容的 Go 1.21+：

```bash
cd ai-agent-learning/examples/go-agent
go test ./...
go vet ./...
go run ./cmd/agent --demo
```

默认 `--demo` 使用 `ScriptedModel`，不需要 API key。真实 HTTP 模式明确要求由环境变量提供 `AGENT_API_KEY`，不会从代码读取密钥。

## 运行 Java 示例

需要 Java 21 和 Maven：

```bash
cd ai-agent-learning/examples/java-agent
mvn test
mvn package
java -jar target/agent-harness-1.0.0.jar --demo
```

Java 示例的单元测试同样使用 `ScriptedModel`；真实请求只在显式传入参数时启用。

## 目录结构

```text
ai-agent-learning/
├── docs.json                 # Mintlify 当前配置
├── style.css                 # 隐藏生产静态导出中的 Hot Reloader 状态组件
├── index.mdx                 # 课程首页
├── PROGRESS.md               # 阶段进度与事实边界
├── VALIDATION.md             # 实际验证记录
├── 00-guide/ ... 20-frameworks/ # 原创教材
├── pi-docs/                  # agent/coding-agent 文档中文参考版
├── reference/                # 源码索引、术语和来源
└── examples/
    ├── go-agent/             # Go 1.21+ 标准库实现
    └── java-agent/           # Java 21 + Maven 实现
```

## 研究基线

- Remote：`https://github.com/coolerks/pi-lean.git`
- Pi commit：`853a80d26c90a14c1886f0ebb8ffaae133ca2185`
- 本地版本优先；官方文档、协议规范和论文只作辅助来源。

教程中的 Pi 路径均相对于仓库根目录。源码行号不是稳定引用；请结合固定 commit 和 symbol 打开源文件。
