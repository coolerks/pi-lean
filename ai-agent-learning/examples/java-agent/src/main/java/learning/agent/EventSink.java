package learning.agent;

@FunctionalInterface
public interface EventSink {
    void accept(Event event);
}
