package learning.agent;

import java.util.ArrayList;
import java.util.List;
import java.util.ServiceLoader;

public final class PluginManager implements AutoCloseable {
    private final List<AgentPlugin> plugins = new ArrayList<>();

    public void load(AgentPlugin plugin, ToolRegistry registry) {
        plugin.register(registry);
        plugins.add(plugin);
    }

    public void loadFromSpi(ToolRegistry registry) {
        ServiceLoader.load(AgentPlugin.class).forEach(plugin -> load(plugin, registry));
    }

    @Override
    public void close() {
        for (int index = plugins.size() - 1; index >= 0; index--) {
            plugins.get(index).close();
        }
    }
}
