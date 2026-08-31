package learning.agent;

public record Response(Message message, String stopReason, Usage usage) {
    public Response {
        if (message == null) {
            throw new IllegalArgumentException("model response message is required");
        }
        stopReason = stopReason == null ? "stop" : stopReason;
        usage = usage == null ? Usage.empty() : usage;
    }
}
