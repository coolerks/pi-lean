package learning.agent;

import com.fasterxml.jackson.databind.ObjectMapper;

import java.io.IOException;
import java.nio.file.Path;
import java.util.List;

public final class Harness implements AutoCloseable {
    private final ObjectMapper mapper;
    private final Agent agent;
    private final ToolRegistry registry;
    private final Session session;
    private final Path sessionPath;
    private final Telemetry telemetry;
    private final PluginManager plugins = new PluginManager();

    public Harness(Model model, Path workspace, Path sessionPath, ObjectMapper mapper) {
        this.mapper = mapper;
        this.sessionPath = sessionPath;
        this.telemetry = new Telemetry();
        PermissionPolicy policy = new PermissionPolicy(workspace);
        ToolRegistry registry = new ToolRegistry();
        this.registry = registry;
        registry.register(BuiltInTools.read(policy));
        registry.register(BuiltInTools.write(policy));
        registry.register(BuiltInTools.edit(policy));
        registry.register(BuiltInTools.grep(policy));
        registry.register(BuiltInTools.shell(policy));
        registry.register(BuiltInTools.calculator());
        LoopOptions options = new LoopOptions(12, ExecutionMode.PARALLEL, 2, 25, null,
                null, event -> {
                    try (Telemetry.SpanScope ignored = telemetry.start("agent." + event.type(),
                            java.util.Map.of("step", Integer.toString(event.step())))) { }
                });
        this.agent = new Agent(new AgentLoop(model, registry, options));
        this.session = new Session(mapper);
    }

    public static Harness load(Model model, Path workspace, Path sessionPath, ObjectMapper mapper) throws IOException {
        Harness harness = new Harness(model, workspace, sessionPath, mapper);
        if (sessionPath != null && java.nio.file.Files.exists(sessionPath)) {
            harness.session.replace(Session.load(sessionPath, mapper).snapshot());
        }
        return harness;
    }

    public RunResult prompt(String text) throws Exception {
        List<Message> history = new java.util.ArrayList<>(session.snapshot());
        history.add(Message.user(text));
        RunResult result = agent.run(history);
        session.replace(result.messages());
        save();
        return result;
    }

    public void compact(int maxTokens) throws IOException {
        session.replace(Compactor.compact(session.snapshot(), maxTokens));
        save();
    }

    public Agent agent() { return agent; }
    public Session session() { return session; }
    public Telemetry telemetry() { return telemetry; }
    public ToolRegistry tools() { return registry; }

    public void loadPlugin(AgentPlugin plugin) {
        plugins.load(plugin, registry);
    }

    private void save() throws IOException {
        if (sessionPath != null) session.save(sessionPath);
    }

    @Override
    public void close() {
        plugins.close();
    }
}
