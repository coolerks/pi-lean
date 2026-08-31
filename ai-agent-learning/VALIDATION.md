# Validation

本文件只记录实际执行过的命令；失败和警告不伪装成成功。

## 环境

| 项目 | 值 |
| --- | --- |
| Pi 研究 commit | `853a80d26c90a14c1886f0ebb8ffaae133ca2185` |
| Remote | `https://github.com/coolerks/pi-lean.git` |
| Node | `v24.19.0` |
| Go | `go1.25.6 darwin/arm64` |
| Java | `21.0.1` |
| Maven | `Apache Maven 3.9.6` |
| Mintlify CLI | `4.2.842`；client `0.0.3535`（`npx --yes mint@latest version`） |

## 文档完整性

| 检查 | 结果 |
| --- | --- |
| `packages/agent/docs/*.md` | 4 个源文件 |
| `ai-agent-learning/pi-docs/agent/*.mdx` | 4 个中文页 |
| `packages/coding-agent/docs/*.md` | 30 个源文件 |
| `ai-agent-learning/pi-docs/coding-agent/*.mdx` | 30 个中文页 |
| `translation-map.json` 数量/路径 | 34/34 通过 |
| 源文档完整原文附录 byte-equivalence | 34/34 通过 |
| 原创/参考 MDX 总数 | 138 |
| `docs.json` 导航页 | 138；脚本检查缺失 0；页面与导航一一对应 |
| Mermaid block | 30 个页面含 Mermaid（原创关键图均已覆盖） |

## 已执行命令

| 命令 | 结果 |
| --- | --- |
| `cd ai-agent-learning && npx --yes mint@latest validate` | 通过；修复过 `List<Message>` 的 MDX angle bracket、裸 URL 和 Session 表格中的 angle bracket |
| `cd ai-agent-learning && npx --yes mint@latest broken-links` | 通过；无 broken links |
| `cd ai-agent-learning && npx --yes mint@latest a11y` | MDX/图片检查通过；CLI 检查 140 个 MDX（仓库 138 个页面，CLI 还计入 2 个入口）；颜色总体 WARN（primary AA，dark color 已达到暗色背景 3:1；建议 AAA 仍待品牌设计） |
| `cd ai-agent-learning && npx --yes mint@latest dev` + `curl -fsS http://127.0.0.1:3000/` | 通过；首页和 `/00-guide/reading-path` 返回本地 HTML；随后已停止 dev server |
| `cd examples/go-agent && go test ./...` | 通过 |
| `cd examples/go-agent && go test -race ./...` | 通过 |
| `cd examples/go-agent && go vet ./...` | 通过 |
| `cd examples/go-agent && go run ./cmd/agent --demo` | 通过；ScriptedModel 输出 calculator 5 和 final |
| `cd examples/java-agent && mvn test` | 通过 |
| `cd examples/java-agent && mvn package` | 通过；生成 fat jar |
| `cd examples/java-agent && java -jar target/agent-harness-1.0.0.jar --demo` | 通过；输出 final `2 + 3 = 5` |
| `python3` 导航/manifest/frontmatter/appendix 检查 | 通过；导航 138/138，翻译附录 34/34 byte-equivalent |
| `npm run check`（根依赖安装后） | 失败：`tsgo --noEmit` 在现有 Pi `packages/ai/src/providers/*.models.ts` 首先报告 JSON manifest 为 `unknown`，随后 `packages/ai/test`、`packages/coding-agent/test` 出现大量 models catalog 类型为 `never`/未知模型错误；未修改 Pi 核心或 `models.generated.ts`，保留该真实失败 |

## 覆盖的测试重点

Go/Java 测试使用 `ScriptedModel` 或本地 HTTP/JSON-RPC fake，不访问收费 API。覆盖 tool dispatch、schema required、unknown tool、multiple calls/source order、max steps、retry、abort/cancellation、HTTP/SSE、session JSONL、compaction、Harness persist/reload、plugin/SPI、skill discovery、MCP call、RAG tenant filter 和 Harness demo。

## 未解决/边界

- `npm ci --ignore-scripts` 后已重跑根目录 `npm run check`；首次安装前的尝试另因 `biome: command not found` 失败。安装后仍因现有 models catalog 类型错误失败。这项检查针对 Pi monorepo，不是 Mintlify 构建的替代品；本任务未修改 Pi 核心、测试或生成模型文件。
- `mint a11y` 只有颜色总体 WARN，不是结构性 MDX 错误；已修复原先 dark color 对暗背景低于 3:1 的 FAIL。
- Go/Java PermissionPolicy 是教学级路径/命令门，不是生产 sandbox；示例没有把 API key 写入代码。
- Go/Java 示例组合了可测试 Harness，但没有冒充 `packages/agent/docs/harness.v2.md` 的完整 durable operation ledger、crash recovery 或 exactly-once 外部副作用。
- 没有执行 Vercel 部署、Git push、PR 或线上资源创建。
