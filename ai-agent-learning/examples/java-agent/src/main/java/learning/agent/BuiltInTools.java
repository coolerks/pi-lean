package learning.agent;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardOpenOption;
import java.util.ArrayList;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.concurrent.TimeUnit;
import java.util.stream.Stream;

public final class BuiltInTools {
    private BuiltInTools() { }

    public static Tool read(PermissionPolicy policy) {
        return new SimpleTool("read", "Read a UTF-8 text file in the workspace.", schema("path", "string"), (cancel, args) -> {
            Path path = policy.resolvePath(args.path("path").asText());
            cancel.throwIfCancelled();
            String text = Files.readString(path);
            boolean truncated = text.length() > 32_000;
            if (truncated) text = text.substring(0, 32_000) + "\n[output truncated]";
            return new ToolResult(text, false, false);
        });
    }

    public static Tool write(PermissionPolicy policy) {
        ObjectNode definition = schema("path", "string");
        definition.with("properties").putObject("content").put("type", "string");
        ((ArrayNode) definition.withArray("required")).add("content");
        return new SimpleTool("write", "Write UTF-8 text to a workspace file.", definition, (cancel, args) -> {
            Path path = policy.resolvePath(args.path("path").asText());
            Files.createDirectories(path.getParent());
            Files.writeString(path, args.path("content").asText(), StandardOpenOption.CREATE, StandardOpenOption.TRUNCATE_EXISTING);
            return new ToolResult("wrote " + args.path("path").asText(), false, false);
        });
    }

    public static Tool edit(PermissionPolicy policy) {
        ObjectNode definition = schema("path", "string");
        ObjectNode properties = (ObjectNode) definition.get("properties");
        properties.putObject("oldText").put("type", "string");
        properties.putObject("newText").put("type", "string");
        ((ArrayNode) definition.withArray("required")).add("oldText").add("newText");
        return new SimpleTool("edit", "Replace one unique occurrence in a workspace file.", definition, (cancel, args) -> {
            Path path = policy.resolvePath(args.path("path").asText());
            String current = Files.readString(path);
            String oldText = args.path("oldText").asText();
            int first = current.indexOf(oldText);
            if (first < 0 || first != current.lastIndexOf(oldText)) {
                throw new IllegalArgumentException("oldText must occur exactly once");
            }
            String updated = current.substring(0, first) + args.path("newText").asText() + current.substring(first + oldText.length());
            Files.writeString(path, updated);
            return new ToolResult("edited " + args.path("path").asText(), false, false);
        });
    }

    public static Tool grep(PermissionPolicy policy) {
        ObjectNode definition = schema("query", "string");
        definition.with("properties").putObject("path").put("type", "string");
        return new SimpleTool("grep", "Search text files under the workspace.", definition, (cancel, args) -> {
            String query = args.path("query").asText();
            Path root = args.path("path").asText().isBlank() ? policy.workspace() : policy.resolvePath(args.path("path").asText());
            List<String> hits = new ArrayList<>();
            try (Stream<Path> paths = Files.walk(root)) {
                for (Path path : paths.filter(Files::isRegularFile).toList()) {
                    cancel.throwIfCancelled();
                    byte[] bytes = Files.readAllBytes(path);
                    if (containsZero(bytes)) continue;
                    List<String> lines = Files.readAllLines(path);
                    for (int index = 0; index < lines.size(); index++) {
                        if (lines.get(index).contains(query)) hits.add(path + ":" + (index + 1) + ":" + lines.get(index));
                        if (hits.size() >= 50) break;
                    }
                    if (hits.size() >= 50) break;
                }
            }
            return new ToolResult(String.join("\n", hits), false, false);
        });
    }

    public static Tool shell(PermissionPolicy policy) {
        ObjectNode definition = schema("command", "string");
        return new SimpleTool("shell", "Run a command in the workspace after policy checks.", definition, (cancel, args) -> {
            String command = args.path("command").asText();
            policy.checkCommand(command);
            Process process = new ProcessBuilder("sh", "-c", command).directory(policy.workspace().toFile()).start();
            while (!process.waitFor(10, TimeUnit.MILLISECONDS)) {
                if (cancel.isCancelled()) {
                    process.destroyForcibly();
                    cancel.throwIfCancelled();
                }
            }
            String output = new String(process.getInputStream().readAllBytes(), StandardCharsets.UTF_8)
                    + new String(process.getErrorStream().readAllBytes(), StandardCharsets.UTF_8);
            if (output.length() > 32_000) output = output.substring(0, 32_000) + "\n[output truncated]";
            if (process.exitValue() != 0) return new ToolResult(output, true, false);
            return new ToolResult(output, false, false);
        });
    }

    public static Tool calculator() {
        ObjectNode definition = schema("operation", "string");
        ObjectNode properties = (ObjectNode) definition.get("properties");
        properties.putObject("left").put("type", "number");
        properties.putObject("right").put("type", "number");
        ((ArrayNode) definition.withArray("required")).add("left").add("right");
        return new SimpleTool("calculator", "Add, subtract, multiply, or divide two numbers.", definition, (cancel, args) -> {
            cancel.throwIfCancelled();
            double left = args.path("left").asDouble();
            double right = args.path("right").asDouble();
            double value = switch (args.path("operation").asText().toLowerCase(Locale.ROOT)) {
                case "add" -> left + right;
                case "subtract" -> left - right;
                case "multiply" -> left * right;
                case "divide" -> {
                    if (right == 0) throw new IllegalArgumentException("division by zero");
                    yield left / right;
                }
                default -> throw new IllegalArgumentException("unknown operation");
            };
            String text = value == Math.rint(value) ? Long.toString((long) value) : Double.toString(value);
            return new ToolResult(text, false, false);
        });
    }

    private static ObjectNode schema(String requiredName, String type) {
        ObjectMapper mapper = new ObjectMapper();
        ObjectNode schema = mapper.createObjectNode().put("type", "object");
        schema.putObject("properties").putObject(requiredName).put("type", type);
        schema.putArray("required").add(requiredName);
        return schema;
    }

    private static boolean containsZero(byte[] bytes) {
        for (byte value : bytes) if (value == 0) return true;
        return false;
    }

    @FunctionalInterface
    private interface Executor {
        ToolResult execute(CancellationToken cancellation, JsonNode arguments) throws Exception;
    }

    private record SimpleTool(String name, String description, JsonNode schema, Executor executor) implements Tool {
        @Override public ToolDefinition definition() { return new ToolDefinition(name, description, schema); }
        @Override public ToolResult execute(CancellationToken cancellation, JsonNode arguments) throws Exception {
            if (arguments == null || !arguments.isObject()) throw new IllegalArgumentException("arguments must be an object");
            return executor.execute(cancellation, arguments);
        }
    }
}
