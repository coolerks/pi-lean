package learning.agent;

import java.util.concurrent.CancellationException;

public final class CancellationToken {
    private volatile boolean cancelled;

    public boolean isCancelled() {
        return cancelled;
    }

    public void cancel() {
        cancelled = true;
    }

    public void throwIfCancelled() {
        if (cancelled || Thread.currentThread().isInterrupted()) {
            throw new CancellationException("operation cancelled");
        }
    }
}
