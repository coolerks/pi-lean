package learning.agent;

public record ToolResult(String content, boolean isError, boolean terminate) {
    public ToolResult {
        content = content == null ? "" : content;
    }

    public static ToolResult error(String message) {
        return new ToolResult(message, true, false);
    }
}
