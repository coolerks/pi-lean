package learning.agent;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.concurrent.CancellationException;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

class AgentTest {
    private final ObjectMapper mapper = new ObjectMapper();

    @Test
    void executesToolAndSendsToolResultBack() throws Exception {
        ScriptedModel model = new ScriptedModel()
                .add(new Response(Message.assistant(List.of(new ToolCall("1", "calculator",
                        mapper.readTree("{\"operation\":\"add\",\"left\":2,\"right\":3}")))), "tool_use", Usage.empty()))
                .add(new Response(Message.assistant("5"), "stop", Usage.empty()));
        ToolRegistry registry = new ToolRegistry();
        registry.register(BuiltInTools.calculator());
        AgentLoop loop = new AgentLoop(model, registry, new LoopOptions(4, ExecutionMode.SEQUENTIAL, 1, 0, null, null, null));

        RunResult result = loop.run(List.of(Message.user("2+3")), new CancellationToken(), QueueSource.empty());

        assertEquals("5", result.finalMessage().content());
        assertEquals(2, model.requests().size());
        assertEquals(Message.Role.TOOL, model.requests().get(1).messages().getLast().role());
        assertEquals("5", model.requests().get(1).messages().getLast().content());
    }

    @Test
    void invalidOrUnknownToolDoesNotInvokeTool() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        Tool tool = new Tool() {
            @Override public ToolDefinition definition() {
                return new ToolDefinition("count", "count", mapper.createObjectNode().put("type", "object"));
            }
            @Override public ToolResult execute(CancellationToken cancellation, com.fasterxml.jackson.databind.JsonNode args) {
                calls.incrementAndGet();
                return new ToolResult("called", false, false);
            }
        };
        ScriptedModel model = new ScriptedModel()
                .add(new Response(Message.assistant(List.of(new ToolCall("1", "missing", mapper.createObjectNode()))), "tool_use", Usage.empty()))
                .add(new Response(Message.assistant("handled"), "stop", Usage.empty()));
        ToolRegistry registry = new ToolRegistry();
        registry.register(tool);
        RunResult result = new AgentLoop(model, registry, LoopOptions.defaults())
                .run(List.of(Message.user("go")), new CancellationToken(), QueueSource.empty());
        assertEquals("handled", result.finalMessage().content());
        assertEquals(0, calls.get());
    }

    @Test
    void missingRequiredArgumentDoesNotInvokeTool() throws Exception {
        AtomicInteger calls = new AtomicInteger();
        Tool tool = new Tool() {
            @Override public ToolDefinition definition() {
                var schema = mapper.createObjectNode().put("type", "object");
                schema.putArray("required").add("value");
                return new ToolDefinition("count", "count", schema);
            }
            @Override public ToolResult execute(CancellationToken cancellation, com.fasterxml.jackson.databind.JsonNode args) {
                calls.incrementAndGet();
                return new ToolResult("called", false, false);
            }
        };
        ScriptedModel model = new ScriptedModel()
                .add(new Response(Message.assistant(List.of(new ToolCall("1", "count", mapper.createObjectNode()))), "tool_use", Usage.empty()))
                .add(new Response(Message.assistant("handled"), "stop", Usage.empty()));
        ToolRegistry registry = new ToolRegistry();
        registry.register(tool);
        RunResult result = new AgentLoop(model, registry, LoopOptions.defaults())
                .run(List.of(Message.user("go")), new CancellationToken(), QueueSource.empty());
        assertEquals(0, calls.get());
        assertTrue(result.messages().stream().anyMatch(message -> message.role() == Message.Role.TOOL && message.isError()));
    }

    @Test
    void retryAndMaxStepsAreBounded() throws Exception {
        ScriptedModel model = new ScriptedModel()
                .fail(new IllegalStateException("temporary"))
                .add(new Response(Message.assistant("ok"), "stop", Usage.empty()));
        AgentLoop loop = new AgentLoop(model, new ToolRegistry(), new LoopOptions(1, ExecutionMode.PARALLEL, 2, 1, null, null, null));
        assertEquals("ok", loop.run(List.of(Message.user("hello")), new CancellationToken(), QueueSource.empty()).finalMessage().content());

        ScriptedModel endless = new ScriptedModel()
                .add(new Response(Message.assistant(List.of(new ToolCall("1", "missing", mapper.createObjectNode()))), "tool_use", Usage.empty()));
        assertThrows(IllegalStateException.class, () -> new AgentLoop(endless, new ToolRegistry(), new LoopOptions(1, ExecutionMode.PARALLEL, 1, 0, null, null, null))
                .run(List.of(Message.user("loop")), new CancellationToken(), QueueSource.empty()));
    }

    @Test
    void harnessPersistsAndReloads() throws Exception {
        Path directory = Files.createTempDirectory("harness-test");
        Path sessionPath = directory.resolve("session.jsonl");
        Harness harness = new Harness(
                new ScriptedModel().add(new Response(Message.assistant("ok"), "stop", Usage.empty())),
                directory,
                sessionPath,
                mapper);
        RunResult result = harness.prompt("hello");
        assertEquals("ok", result.finalMessage().content());
        harness.close();

        Harness loaded = Harness.load(new ScriptedModel(), directory, sessionPath, mapper);
        assertEquals(2, loaded.session().snapshot().size());
        assertEquals(Message.Role.USER, loaded.session().snapshot().getFirst().role());
        assertEquals("ok", loaded.session().snapshot().getLast().content());
        loaded.close();
    }

    @Test
    void serviceLoaderDiscoversPlugins() {
        ToolRegistry registry = new ToolRegistry();
        PluginManager manager = new PluginManager();
        manager.loadFromSpi(registry);
        assertNotNull(registry.find("spi_echo"));
        manager.close();
    }

    @Test
    void abortCancelsAnInFlightModel() throws Exception {
        ScriptedModel model = new ScriptedModel().delay(1000)
                .add(new Response(Message.assistant("late"), "stop", Usage.empty()));
        ToolRegistry registry = new ToolRegistry();
        Agent agent = new Agent(new AgentLoop(model, registry, new LoopOptions(1, ExecutionMode.PARALLEL, 1, 0, null, null, null)));
        var executor = Executors.newSingleThreadExecutor();
        Future<RunResult> future = executor.submit(() -> agent.run(List.of(Message.user("wait"))));
        Thread.sleep(30);
        agent.abort();
        ExecutionException failure = assertThrows(ExecutionException.class, future::get);
        assertTrue(failure.getCause() instanceof CancellationException);
        executor.shutdownNow();
    }

    @Test
    void sessionAndSkillPersistenceWork() throws Exception {
        Path directory = Files.createTempDirectory("agent-test");
        Path sessionPath = directory.resolve("session.jsonl");
        Session session = new Session(mapper);
        session.append(Message.user("one"), Message.assistant("two"));
        session.save(sessionPath);
        assertEquals(2, Session.load(sessionPath, mapper).snapshot().size());
        assertFalse(Compactor.compact(List.of(Message.user("a long message"), Message.assistant("another long response")), 2).isEmpty());

        Path skill = directory.resolve("review/SKILL.md");
        Files.createDirectories(skill.getParent());
        Files.writeString(skill, "---\nname: review\ndescription: review code\n---\n# details\n");
        assertEquals("review", SkillDiscovery.discover(directory).getFirst().name());
    }
}
