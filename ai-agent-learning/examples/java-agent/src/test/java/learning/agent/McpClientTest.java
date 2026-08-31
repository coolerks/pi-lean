package learning.agent;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;

import java.io.BufferedReader;
import java.io.BufferedWriter;
import java.io.IOException;
import java.io.InputStreamReader;
import java.io.OutputStreamWriter;
import java.io.PipedInputStream;
import java.io.PipedOutputStream;
import java.nio.charset.StandardCharsets;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;

class McpClientTest {
    @Test
    void listsAndCallsRemoteToolOverJsonLines() throws Exception {
        PipedInputStream clientInput = new PipedInputStream();
        PipedOutputStream serverOutput = new PipedOutputStream(clientInput);
        PipedInputStream serverInput = new PipedInputStream();
        PipedOutputStream clientOutput = new PipedOutputStream(serverInput);
        Thread server = new Thread(() -> serve(serverInput, serverOutput));
        server.start();

        McpClient client = new McpClient(clientInput, clientOutput, new ObjectMapper());
        try {
            List<Tool> tools = client.listTools(new CancellationToken());
            assertEquals(1, tools.size());
            assertEquals("remote_echo", tools.getFirst().definition().name());
            ToolResult result = tools.getFirst().execute(new CancellationToken(), new ObjectMapper().createObjectNode());
            assertEquals("remote result", result.content());
        } finally {
            client.close();
            server.join(1000);
        }
    }

    private static void serve(PipedInputStream input, PipedOutputStream output) {
        try (BufferedReader reader = new BufferedReader(new InputStreamReader(input, StandardCharsets.UTF_8));
             BufferedWriter writer = new BufferedWriter(new OutputStreamWriter(output, StandardCharsets.UTF_8))) {
            String line;
            while ((line = reader.readLine()) != null) {
                var request = new ObjectMapper().readTree(line);
                String method = request.path("method").asText();
                String result = method.equals("tools/list")
                        ? "{\"tools\":[{\"name\":\"remote_echo\",\"description\":\"echo\",\"inputSchema\":{\"type\":\"object\"}}]}"
                        : "{\"content\":[{\"type\":\"text\",\"text\":\"remote result\"}]}";
                writer.write("{\"jsonrpc\":\"2.0\",\"id\":" + request.path("id").asInt() + ",\"result\":" + result + "}");
                writer.newLine();
                writer.flush();
            }
        } catch (IOException ignored) {
            // Closing the client is the expected way for this test server to stop.
        }
    }
}
