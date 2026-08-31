package learning.agent;

import com.fasterxml.jackson.databind.JsonNode;

public record ToolDefinition(String name, String description, JsonNode schema) {
    public ToolDefinition {
        if (name == null || name.isBlank()) {
            throw new IllegalArgumentException("tool name is required");
        }
        description = description == null ? "" : description;
        if (schema == null || !schema.isObject()) {
            throw new IllegalArgumentException("tool schema must be an object");
        }
    }
}
