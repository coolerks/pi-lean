package learning.agent;

public record LoopOptions(
        int maxSteps,
        ExecutionMode executionMode,
        int maxAttempts,
        long baseDelayMillis,
        BeforeToolHook beforeTool,
        AfterToolHook afterTool,
        EventSink events) {
    public LoopOptions {
        maxSteps = maxSteps <= 0 ? 16 : maxSteps;
        executionMode = executionMode == null ? ExecutionMode.PARALLEL : executionMode;
        maxAttempts = maxAttempts <= 0 ? 1 : maxAttempts;
        events = events == null ? event -> { } : events;
    }

    public static LoopOptions defaults() {
        return new LoopOptions(16, ExecutionMode.PARALLEL, 1, 50, null, null, event -> { });
    }
}
