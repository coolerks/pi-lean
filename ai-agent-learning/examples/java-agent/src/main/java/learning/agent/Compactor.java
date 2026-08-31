package learning.agent;

import java.util.ArrayList;
import java.util.List;

public final class Compactor {
    private Compactor() { }

    public static int estimateTokens(List<Message> messages) {
        return messages.stream().mapToInt(message -> message.content().length() + message.toolCalls().stream()
                .mapToInt(call -> call.name().length() + call.arguments().toString().length()).sum()).sum() / 4 + 1;
    }

    public static List<Message> compact(List<Message> messages, int maxTokens) {
        if (maxTokens <= 0) {
            throw new IllegalArgumentException("maxTokens must be positive");
        }
        if (estimateTokens(messages) <= maxTokens) {
            return List.copyOf(messages);
        }
        List<Message> result = new ArrayList<>();
        int start = 0;
        if (!messages.isEmpty() && messages.get(0).role() == Message.Role.SYSTEM) {
            result.add(messages.get(0));
            start = 1;
        }
        int keepFrom = Math.max(start, messages.size() - 4);
        result.add(Message.assistant("[compaction summary: " + (keepFrom - start)
                + " earlier messages omitted; verify current files before acting]"));
        result.addAll(messages.subList(keepFrom, messages.size()));
        return List.copyOf(result);
    }
}
