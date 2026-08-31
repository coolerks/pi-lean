package learning.agent;

import com.fasterxml.jackson.databind.JsonNode;

public record ToolCall(String id, String name, JsonNode arguments) {
    public ToolCall {
        if (id == null || id.isBlank()) {
            throw new IllegalArgumentException("tool call id is required");
        }
        if (name == null || name.isBlank()) {
            throw new IllegalArgumentException("tool call name is required");
        }
        if (arguments == null || !arguments.isObject()) {
            throw new IllegalArgumentException("tool arguments must be a JSON object");
        }
    }
}
