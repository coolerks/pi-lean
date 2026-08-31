# Java Agent 示例

Java 21 + Maven。基础实现使用 `java.net.http.HttpClient`、Jackson、`ExecutorService`/`Future`、Listener 和 `ServiceLoader` SPI；没有 Spring AI、LangChain4j 等框架。

```bash
mvn test
mvn package
java -jar target/agent-harness-1.0.0.jar --demo
```

阶段与源码：

| 阶段 | 文件/能力 |
| --- | --- |
| Java 01 | `HttpModel.complete`：HTTP JSON |
| Java 02 | `HttpModel.stream`：SSE 行流 |
| Java 03 | `Tool`、`ToolDefinition`、`ToolRegistry`：schema gate |
| Java 04 | `AgentLoop`：有限循环和 Tool Result |
| Java 05 | `BuiltInTools`：read/write/edit/grep/shell |
| Java 06 | `Message`、`Request`：Context 边界 |
| Java 07 | `CancellationToken`、retry、`Future` interruption |
| Java 08 | `EventSink`：事件和 Telemetry |
| Java 09 | `Session`：JSONL 保存与恢复 |
| Java 10 | `Compactor`：token estimate 和 summary |
| Java 11 | `Agent`、`QueueSource`：steering/follow-up |
| Java 12 | `AgentPlugin`、`PluginManager`：SPI/registry |
| Java 13 | `SkillDiscovery`：SKILL.md metadata |
| Java 14 | `McpClient`：JSON-RPC stdio client |
| Java 15 | `Harness`、`Telemetry`：组合层 |

Jackson 只用于 JSON AST/序列化，依赖版本在 `pom.xml` 精确声明。`PermissionPolicy` 和 stdio client 是教学实现，不能替代容器隔离；durable operation log、崩溃恢复和副作用幂等见 `19-build-harness`。
