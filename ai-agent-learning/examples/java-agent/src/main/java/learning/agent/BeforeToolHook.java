package learning.agent;

@FunctionalInterface
public interface BeforeToolHook {
    void check(CancellationToken cancellation, ToolCall call, ToolDefinition definition) throws Exception;
}
