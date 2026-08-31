package learning.agent;

public record Event(String type, int step, String detail, long timestamp) {
    public Event(String type, int step, String detail) {
        this(type, step, detail, System.currentTimeMillis());
    }
}
