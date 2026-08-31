package learning.agent;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.io.BufferedReader;
import java.io.BufferedWriter;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.io.OutputStreamWriter;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;

// Minimal JSON-RPC-over-stdio client. It serializes calls; a production client
// needs a response dispatcher for concurrent request ids and notifications.
public final class McpClient implements AutoCloseable {
    private final ObjectMapper mapper;
    private final BufferedReader reader;
    private final BufferedWriter writer;
    private final Process process;
    private int nextId = 1;

    public McpClient(InputStream input, OutputStream output, ObjectMapper mapper) {
        this(input, output, null, mapper);
    }

    private McpClient(InputStream input, OutputStream output, Process process, ObjectMapper mapper) {
        this.reader = new BufferedReader(new InputStreamReader(input, StandardCharsets.UTF_8));
        this.writer = new BufferedWriter(new OutputStreamWriter(output, StandardCharsets.UTF_8));
        this.process = process;
        this.mapper = mapper;
    }

    public static McpClient startStdio(ObjectMapper mapper, String command, String... args) throws IOException {
        List<String> commandLine = new ArrayList<>();
        commandLine.add(command);
        commandLine.addAll(List.of(args));
        Process process = new ProcessBuilder(commandLine).redirectError(ProcessBuilder.Redirect.INHERIT).start();
        return new McpClient(process.getInputStream(), process.getOutputStream(), process, mapper);
    }

    public synchronized JsonNode call(String method, JsonNode params, CancellationToken cancellation) throws IOException {
        cancellation.throwIfCancelled();
        int id = nextId++;
        ObjectNode request = mapper.createObjectNode().put("jsonrpc", "2.0").put("id", id).put("method", method);
        request.set("params", params == null ? mapper.createObjectNode() : params);
        writer.write(mapper.writeValueAsString(request));
        writer.newLine();
        writer.flush();
        String line;
        while ((line = reader.readLine()) != null) {
            JsonNode response = mapper.readTree(line);
            if (!response.has("id") || response.path("id").asInt(-1) != id) continue;
            if (response.has("error")) throw new IOException("MCP error: " + response.path("error"));
            return response.path("result");
        }
        throw new IOException("MCP server closed stdout");
    }

    public List<Tool> listTools(CancellationToken cancellation) throws IOException {
        JsonNode result = call("tools/list", mapper.createObjectNode(), cancellation);
        List<Tool> tools = new ArrayList<>();
        for (JsonNode item : result.path("tools")) {
            String name = item.path("name").asText();
            if (!name.isBlank()) {
                tools.add(new RemoteTool(this, name, item.path("description").asText(), item.path("inputSchema"), cancellation));
            }
        }
        return tools;
    }

    @Override
    public synchronized void close() throws IOException {
        reader.close();
        writer.close();
        if (process != null) process.destroy();
    }

    private static final class RemoteTool implements Tool {
        private final McpClient client;
        private final String name;
        private final String description;
        private final JsonNode schema;
        private final CancellationToken token;

        private RemoteTool(McpClient client, String name, String description, JsonNode schema, CancellationToken token) {
            this.client = client;
            this.name = name;
            this.description = description;
            this.schema = schema;
            this.token = token;
        }

        @Override public ToolDefinition definition() { return new ToolDefinition(name, description, schema); }

        @Override
        public ToolResult execute(CancellationToken cancellation, JsonNode arguments) throws Exception {
            ObjectNode params = client.mapper.createObjectNode().put("name", name);
            params.set("arguments", arguments);
            JsonNode result = client.call("tools/call", params, cancellation == null ? token : cancellation);
            StringBuilder content = new StringBuilder();
            for (JsonNode item : result.path("content")) {
                if (item.path("type").asText().equals("text")) content.append(item.path("text").asText());
            }
            return new ToolResult(content.toString(), result.path("isError").asBoolean(false), false);
        }
    }
}
