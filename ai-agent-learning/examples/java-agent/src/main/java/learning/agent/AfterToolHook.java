package learning.agent;

@FunctionalInterface
public interface AfterToolHook {
    ToolResult apply(CancellationToken cancellation, ToolCall call, ToolResult result) throws Exception;
}
