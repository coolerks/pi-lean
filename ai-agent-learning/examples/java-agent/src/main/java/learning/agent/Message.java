package learning.agent;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.databind.JsonNode;

import java.util.List;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public record Message(
        Role role,
        String content,
        List<ToolCall> toolCalls,
        String toolCallId,
        String toolName,
        boolean isError,
        long timestamp) {
    public Message {
        if (role == null) {
            throw new IllegalArgumentException("role is required");
        }
        content = content == null ? "" : content;
        toolCalls = toolCalls == null ? List.of() : List.copyOf(toolCalls);
        if (timestamp == 0) {
            timestamp = System.currentTimeMillis();
        }
    }

    public static Message user(String content) {
        return new Message(Role.USER, content, List.of(), null, null, false, 0);
    }

    public static Message assistant(String content) {
        return new Message(Role.ASSISTANT, content, List.of(), null, null, false, 0);
    }

    public static Message assistant(List<ToolCall> calls) {
        return new Message(Role.ASSISTANT, "", calls, null, null, false, 0);
    }

    public static Message tool(ToolCall call, ToolResult result) {
        return new Message(Role.TOOL, result.content(), List.of(), call.id(), call.name(), result.isError(), 0);
    }

    public enum Role {
        SYSTEM, USER, ASSISTANT, TOOL
    }
}
