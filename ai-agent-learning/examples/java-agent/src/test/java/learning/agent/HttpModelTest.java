package learning.agent;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.http.HttpClient;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

class HttpModelTest {
    @Test
    void sendsJsonAndConsumesCompleteAndSseResponses() throws Exception {
        AtomicReference<String> authorization = new AtomicReference<>();
        HttpServer server = HttpServer.create(new InetSocketAddress("localhost", 0), 0);
        server.createContext("/model", exchange -> handle(exchange, authorization));
        server.start();
        try {
            ObjectMapper mapper = new ObjectMapper();
            String endpoint = "http://localhost:" + server.getAddress().getPort() + "/model";
            HttpModel model = new HttpModel(HttpClient.newHttpClient(), mapper, endpoint, "test-key", "test-model");
            Request request = new Request(java.util.List.of(Message.user("hello")), java.util.List.of());

            Response complete = model.complete(request, new CancellationToken());
            assertEquals("hello back", complete.message().content());
            assertEquals("Bearer test-key", authorization.get());

            StringBuilder deltas = new StringBuilder();
            Response streamed = model.stream(request, new CancellationToken(), deltas::append);
            assertEquals("hello", deltas.toString());
            assertEquals("stop", streamed.stopReason());
        } finally {
            server.stop(0);
        }
    }

    private static void handle(HttpExchange exchange, AtomicReference<String> authorization) throws IOException {
        authorization.set(exchange.getRequestHeaders().getFirst("Authorization"));
        String request = new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8);
        exchange.getResponseHeaders().set("Content-Type", "application/json");
        byte[] response;
        if (request.contains("\"stream\":true")) {
            exchange.getResponseHeaders().set("Content-Type", "text/event-stream");
            response = ("data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n"
                    + "data: {\"choices\":[{\"delta\":{\"content\":\"lo\"},\"finish_reason\":\"stop\"}]}\n\n"
                    + "data: [DONE]\n\n").getBytes(StandardCharsets.UTF_8);
        } else {
            response = "{\"choices\":[{\"message\":{\"role\":\"assistant\",\"content\":\"hello back\"},\"finish_reason\":\"stop\"}]}".getBytes(StandardCharsets.UTF_8);
        }
        exchange.sendResponseHeaders(200, response.length);
        exchange.getResponseBody().write(response);
        exchange.close();
    }

    @Test
    void rejectsResponseWithoutChoices() throws Exception {
        HttpServer server = HttpServer.create(new InetSocketAddress("localhost", 0), 0);
        server.createContext("/model", exchange -> {
            byte[] response = "{\"choices\":[]}".getBytes(StandardCharsets.UTF_8);
            exchange.sendResponseHeaders(200, response.length);
            exchange.getResponseBody().write(response);
            exchange.close();
        });
        server.start();
        try {
            String endpoint = "http://localhost:" + server.getAddress().getPort() + "/model";
            HttpModel model = new HttpModel(HttpClient.newHttpClient(), new ObjectMapper(), endpoint, "", "test-model");
            Exception failure = org.junit.jupiter.api.Assertions.assertThrows(Exception.class,
                    () -> model.complete(new Request(java.util.List.of(), java.util.List.of()), new CancellationToken()));
            assertTrue(failure.getMessage().contains("no choices"));
        } finally {
            server.stop(0);
        }
    }
}
