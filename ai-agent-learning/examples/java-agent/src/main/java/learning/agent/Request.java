package learning.agent;

import java.util.List;

public record Request(List<Message> messages, List<ToolDefinition> tools) {
    public Request {
        messages = List.copyOf(messages == null ? List.of() : messages);
        tools = List.copyOf(tools == null ? List.of() : tools);
    }
}
