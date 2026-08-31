package learning.agent;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ArrayNode;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

public final class ToolRegistry {
    private final Map<String, Tool> tools = new LinkedHashMap<>();

    public synchronized void register(Tool tool) {
        String name = tool.definition().name();
        if (tools.containsKey(name)) {
            throw new IllegalArgumentException("tool already registered: " + name);
        }
        tools.put(name, tool);
    }

    public synchronized Tool find(String name) {
        return tools.get(name);
    }

    public synchronized List<ToolDefinition> definitions() {
        return tools.values().stream().map(Tool::definition).toList();
    }

    public Tool validate(ToolCall call) {
        Tool tool = find(call.name());
        if (tool == null) {
            throw new IllegalArgumentException("unknown tool: " + call.name());
        }
        JsonNode required = tool.definition().schema().get("required");
        if (required instanceof ArrayNode requiredNames) {
            for (JsonNode name : requiredNames) {
                if (!call.arguments().has(name.asText())) {
                    throw new IllegalArgumentException("missing required argument: " + name.asText());
                }
            }
        }
        return tool;
    }

    public List<Tool> snapshot() {
        return new ArrayList<>(tools.values());
    }
}
