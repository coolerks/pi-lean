package learning.agent;

import java.util.ArrayDeque;
import java.util.ArrayList;
import java.util.Deque;
import java.util.List;
import java.util.concurrent.CancellationException;

public final class ScriptedModel implements Model {
    private final Deque<Response> responses = new ArrayDeque<>();
    private final Deque<Exception> failures = new ArrayDeque<>();
    private final List<Request> requests = new ArrayList<>();
    private long delayMillis;

    public synchronized ScriptedModel add(Response response) {
        responses.add(response);
        return this;
    }

    public synchronized ScriptedModel fail(Exception failure) {
        failures.add(failure);
        return this;
    }

    public synchronized ScriptedModel delay(long millis) {
        delayMillis = millis;
        return this;
    }

    public synchronized List<Request> requests() {
        return List.copyOf(requests);
    }

    @Override
    public Response complete(Request request, CancellationToken cancellation) throws Exception {
        long delay;
        synchronized (this) {
            requests.add(request);
            delay = delayMillis;
        }
        long deadline = System.currentTimeMillis() + delay;
        while (System.currentTimeMillis() < deadline) {
            cancellation.throwIfCancelled();
            Thread.sleep(Math.min(10, Math.max(1, deadline - System.currentTimeMillis())));
        }
        cancellation.throwIfCancelled();
        synchronized (this) {
            if (!failures.isEmpty()) {
                throw failures.removeFirst();
            }
            if (responses.isEmpty()) {
                throw new IllegalStateException("scripted model has no response left");
            }
            return responses.removeFirst();
        }
    }
}
