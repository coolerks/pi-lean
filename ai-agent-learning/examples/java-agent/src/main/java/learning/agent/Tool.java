package learning.agent;

import com.fasterxml.jackson.databind.JsonNode;

public interface Tool {
    ToolDefinition definition();

    ToolResult execute(CancellationToken cancellation, JsonNode arguments) throws Exception;
}
