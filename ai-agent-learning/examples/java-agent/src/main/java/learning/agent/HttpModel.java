package learning.agent;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.function.Consumer;

// A small OpenAI-compatible Chat Completions adapter. Other providers belong
// behind their own adapter and must not leak types into AgentLoop.
public final class HttpModel implements Model, StreamingModel {
    private final HttpClient client;
    private final ObjectMapper mapper;
    private final URI endpoint;
    private final String apiKey;
    private final String modelId;

    public HttpModel(String endpoint, String apiKey, String modelId) {
        this(HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(10)).build(), new ObjectMapper(), endpoint, apiKey, modelId);
    }

    public HttpModel(HttpClient client, ObjectMapper mapper, String endpoint, String apiKey, String modelId) {
        this.client = client;
        this.mapper = mapper;
        this.endpoint = URI.create(endpoint);
        this.apiKey = apiKey;
        this.modelId = modelId;
    }

    @Override
    public Response complete(Request request, CancellationToken cancellation) throws Exception {
        cancellation.throwIfCancelled();
        String body = mapper.writeValueAsString(payload(request, false));
        HttpRequest httpRequest = HttpRequest.newBuilder(endpoint)
                .timeout(Duration.ofMinutes(2))
                .header("Content-Type", "application/json")
                .header("Authorization", "Bearer " + apiKey)
                .POST(HttpRequest.BodyPublishers.ofString(body))
                .build();
        HttpResponse<String> response = client.send(httpRequest, HttpResponse.BodyHandlers.ofString());
        if (response.statusCode() < 200 || response.statusCode() >= 300) {
            throw new IllegalStateException("model request failed (" + response.statusCode() + "): " + response.body());
        }
        return decode(mapper.readTree(response.body()));
    }

    @Override
    public Response stream(Request request, CancellationToken cancellation, Consumer<String> onDelta) throws Exception {
        cancellation.throwIfCancelled();
        String body = mapper.writeValueAsString(payload(request, true));
        HttpRequest httpRequest = HttpRequest.newBuilder(endpoint)
                .timeout(Duration.ofMinutes(2))
                .header("Content-Type", "application/json")
                .header("Accept", "text/event-stream")
                .header("Authorization", "Bearer " + apiKey)
                .POST(HttpRequest.BodyPublishers.ofString(body))
                .build();
        HttpResponse<java.util.stream.Stream<String>> response = client.send(httpRequest, HttpResponse.BodyHandlers.ofLines());
        if (response.statusCode() < 200 || response.statusCode() >= 300) {
            throw new IllegalStateException("stream request failed (" + response.statusCode() + ")");
        }
        StringBuilder content = new StringBuilder();
        String finish = "stop";
        try (java.util.stream.Stream<String> lines = response.body()) {
            for (String line : (Iterable<String>) lines::iterator) {
                cancellation.throwIfCancelled();
                if (!line.startsWith("data:")) {
                    continue;
                }
                String data = line.substring("data:".length()).trim();
                if (data.equals("[DONE]")) {
                    break;
                }
                JsonNode chunk = mapper.readTree(data);
                JsonNode choice = chunk.path("choices").path(0);
                String delta = choice.path("delta").path("content").asText("");
                if (!delta.isEmpty()) {
                    content.append(delta);
                    if (onDelta != null) {
                        onDelta.accept(delta);
                    }
                }
                if (!choice.path("finish_reason").isMissingNode() && !choice.path("finish_reason").isNull()) {
                    finish = choice.path("finish_reason").asText(finish);
                }
            }
        }
        return new Response(Message.assistant(content.toString()), finish, Usage.empty());
    }

    private ObjectNode payload(Request request, boolean stream) {
        ObjectNode root = mapper.createObjectNode().put("model", modelId).put("stream", stream);
        ArrayNode messages = root.putArray("messages");
        for (Message message : request.messages()) {
            ObjectNode item = messages.addObject();
            item.put("role", message.role().name().toLowerCase());
            item.put("content", message.content());
            if (message.toolCallId() != null) {
                item.put("tool_call_id", message.toolCallId());
            }
            if (message.toolName() != null) {
                item.put("name", message.toolName());
            }
            if (!message.toolCalls().isEmpty()) {
                ArrayNode calls = item.putArray("tool_calls");
                for (ToolCall call : message.toolCalls()) {
                    ObjectNode encoded = calls.addObject().put("id", call.id()).put("type", "function");
                    encoded.putObject("function").put("name", call.name()).put("arguments", call.arguments().toString());
                }
            }
        }
        if (!request.tools().isEmpty()) {
            ArrayNode tools = root.putArray("tools");
            for (ToolDefinition definition : request.tools()) {
                ObjectNode function = tools.addObject().put("type", "function").putObject("function");
                function.put("name", definition.name()).put("description", definition.description());
                function.set("parameters", definition.schema());
            }
        }
        return root;
    }

    private Response decode(JsonNode root) throws Exception {
        JsonNode choices = root.path("choices");
        if (!choices.isArray() || choices.isEmpty() || !choices.path(0).isObject()) {
            throw new IllegalStateException("model response has no choices");
        }
        JsonNode choice = choices.path(0);
        JsonNode encodedMessage = choice.path("message");
        if (!encodedMessage.isObject()) {
            throw new IllegalStateException("model response choice has no message");
        }
        List<ToolCall> calls = new ArrayList<>();
        for (JsonNode encodedCall : encodedMessage.path("tool_calls")) {
            JsonNode function = encodedCall.path("function");
            String arguments = function.path("arguments").asText("{}");
            calls.add(new ToolCall(encodedCall.path("id").asText(), function.path("name").asText(), mapper.readTree(arguments)));
        }
        Message message = calls.isEmpty()
                ? Message.assistant(encodedMessage.path("content").asText(""))
                : Message.assistant(calls);
        JsonNode usage = root.path("usage");
        Usage tokenUsage = new Usage(usage.path("prompt_tokens").asInt(), usage.path("completion_tokens").asInt(), usage.path("total_tokens").asInt(), 0);
        return new Response(message, choice.path("finish_reason").asText("stop"), tokenUsage);
    }
}
