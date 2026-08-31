package learning.agent;

import java.util.List;

public record RunResult(List<Message> messages, Message finalMessage, int steps) {
    public RunResult {
        messages = List.copyOf(messages);
    }
}
