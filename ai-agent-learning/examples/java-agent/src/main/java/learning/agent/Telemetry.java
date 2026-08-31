package learning.agent;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

public final class Telemetry {
    public record Span(String name, long startedAt, long finishedAt, Map<String, String> fields, String error) { }

    private final List<Span> spans = new ArrayList<>();

    public synchronized SpanScope start(String name, Map<String, String> fields) {
        return new SpanScope(this, name, fields);
    }

    public synchronized List<Span> snapshot() {
        return List.copyOf(spans);
    }

    public static final class SpanScope implements AutoCloseable {
        private final Telemetry telemetry;
        private final String name;
        private final long startedAt = System.currentTimeMillis();
        private final Map<String, String> fields;
        private String error;

        private SpanScope(Telemetry telemetry, String name, Map<String, String> fields) {
            this.telemetry = telemetry;
            this.name = name;
            this.fields = fields;
        }

        public SpanScope failed(Exception exception) {
            error = exception.getMessage();
            return this;
        }

        @Override
        public void close() {
            synchronized (telemetry) {
                telemetry.spans.add(new Span(name, startedAt, System.currentTimeMillis(), fields, error));
            }
        }
    }
}
