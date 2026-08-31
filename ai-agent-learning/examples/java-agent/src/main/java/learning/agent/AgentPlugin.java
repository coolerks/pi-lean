package learning.agent;

public interface AgentPlugin extends AutoCloseable {
    String name();

    void register(ToolRegistry registry);

    @Override
    default void close() { }
}
