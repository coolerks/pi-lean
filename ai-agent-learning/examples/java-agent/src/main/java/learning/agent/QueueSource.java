package learning.agent;

import java.util.List;

public interface QueueSource {
    List<Message> steering();

    List<Message> followUps();

    static QueueSource empty() {
        return new QueueSource() {
            @Override public List<Message> steering() { return List.of(); }
            @Override public List<Message> followUps() { return List.of(); }
        };
    }
}
