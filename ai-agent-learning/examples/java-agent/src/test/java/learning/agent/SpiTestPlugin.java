package learning.agent;

import com.fasterxml.jackson.databind.node.JsonNodeFactory;

public final class SpiTestPlugin implements AgentPlugin {
    @Override
    public String name() {
        return "spi-test";
    }

    @Override
    public void register(ToolRegistry registry) {
        registry.register(new Tool() {
            @Override
            public ToolDefinition definition() {
                return new ToolDefinition("spi_echo", "echo from SPI", JsonNodeFactory.instance.objectNode());
            }

            @Override
            public ToolResult execute(CancellationToken cancellation, com.fasterxml.jackson.databind.JsonNode arguments) {
                return new ToolResult("spi", false, false);
            }
        });
    }
}
