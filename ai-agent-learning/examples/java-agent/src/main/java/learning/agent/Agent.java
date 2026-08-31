package learning.agent;

import java.util.ArrayList;
import java.util.List;

// Lifecycle and queues around AgentLoop. Queue messages are inserted only at
// the boundary before a new model request.
public final class Agent {
    private final AgentLoop loop;
    private final List<Message> steering = new ArrayList<>();
    private final List<Message> followUps = new ArrayList<>();
    private final List<Message> nextRun = new ArrayList<>();
    private boolean running;
    private CancellationToken currentCancellation;

    public Agent(AgentLoop loop) {
        this.loop = loop;
    }

    public RunResult prompt(String text) throws Exception {
        List<Message> initial;
        CancellationToken cancellation;
        synchronized (this) {
            ensureIdle();
            initial = new ArrayList<>(nextRun);
            nextRun.clear();
            initial.add(Message.user(text));
            cancellation = beginRun();
        }
        return execute(initial, cancellation);
    }

    public RunResult run(List<Message> history) throws Exception {
        CancellationToken cancellation;
        synchronized (this) {
            ensureIdle();
            cancellation = beginRun();
        }
        return execute(history, cancellation);
    }

    private RunResult execute(List<Message> history, CancellationToken cancellation) throws Exception {
        try {
            return loop.run(history, cancellation, new QueueSource() {
                @Override
                public List<Message> steering() {
                    return drainSteering();
                }

                @Override
                public List<Message> followUps() {
                    return drainFollowUps();
                }
            });
        } finally {
            synchronized (this) {
                running = false;
                currentCancellation = null;
            }
        }
    }

    public synchronized void steer(String text) {
        ensureRunning();
        steering.add(Message.user(text));
    }

    public synchronized void followUp(String text) {
        ensureRunning();
        followUps.add(Message.user(text));
    }

    public synchronized void nextRun(String text) {
        nextRun.add(Message.user(text));
    }

    public synchronized void abort() {
        ensureRunning();
        steering.clear();
        followUps.clear();
        currentCancellation.cancel();
    }

    public synchronized boolean isRunning() {
        return running;
    }

    private CancellationToken beginRun() {
        CancellationToken cancellation = new CancellationToken();
        running = true;
        currentCancellation = cancellation;
        return cancellation;
    }

    private synchronized void ensureIdle() {
        if (running) {
            throw new IllegalStateException("agent is already running");
        }
    }

    private synchronized void ensureRunning() {
        if (!running || currentCancellation == null) {
            throw new IllegalStateException("queue requires an active run");
        }
    }

    private synchronized List<Message> drainSteering() {
        List<Message> result = new ArrayList<>(steering);
        steering.clear();
        return result;
    }

    private synchronized List<Message> drainFollowUps() {
        List<Message> result = new ArrayList<>(followUps);
        followUps.clear();
        return result;
    }
}
