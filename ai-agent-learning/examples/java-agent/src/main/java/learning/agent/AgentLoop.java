package learning.agent;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.CancellationException;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;

public final class AgentLoop {
    private final Model model;
    private final ToolRegistry registry;
    private final LoopOptions options;

    public AgentLoop(Model model, ToolRegistry registry, LoopOptions options) {
        this.model = model;
        this.registry = registry;
        this.options = options == null ? LoopOptions.defaults() : options;
    }

    public RunResult run(List<Message> initial, CancellationToken cancellation, QueueSource queue) throws Exception {
        if (model == null || registry == null) {
            throw new IllegalStateException("model and registry are required");
        }
        CancellationToken token = cancellation == null ? new CancellationToken() : cancellation;
        QueueSource source = queue == null ? QueueSource.empty() : queue;
        List<Message> history = new ArrayList<>(initial == null ? List.of() : initial);
        Message finalMessage = null;

        for (int step = 1; step <= options.maxSteps(); step++) {
            List<Message> steering = source.steering();
            history.addAll(steering);
            for (Message message : steering) {
                emit(new Event("steering", step, message.content()));
            }
            emit(new Event("step_start", step, ""));
            Response response = completeWithRetry(new Request(history, registry.definitions()), token, step);
            Message assistant = response.message();
            history.add(assistant);
            finalMessage = assistant;
            emit(new Event("assistant", step, assistant.content()));

            if (assistant.toolCalls().isEmpty()) {
                List<Message> followUps = source.followUps();
                if (followUps.isEmpty()) {
                    emit(new Event("run_end", step, response.stopReason()));
                    return new RunResult(history, finalMessage, step);
                }
                history.addAll(followUps);
                for (Message message : followUps) {
                    emit(new Event("follow_up", step, message.content()));
                }
                continue;
            }

            List<ToolResult> results = executeBatch(assistant, token, step);
            boolean terminate = true;
            for (int index = 0; index < results.size(); index++) {
                ToolResult result = results.get(index);
                ToolCall call = assistant.toolCalls().get(index);
                history.add(Message.tool(call, result));
                if (!result.terminate()) {
                    terminate = false;
                }
            }
            if (terminate) {
                List<Message> followUps = source.followUps();
                if (followUps.isEmpty()) {
                    emit(new Event("run_end", step, "tool_batch_terminated"));
                    return new RunResult(history, finalMessage, step);
                }
                history.addAll(followUps);
            }
        }
        throw new IllegalStateException("max steps exceeded: " + options.maxSteps());
    }

    private Response completeWithRetry(Request request, CancellationToken cancellation, int step) throws Exception {
        Exception last = null;
        for (int attempt = 1; attempt <= options.maxAttempts(); attempt++) {
            cancellation.throwIfCancelled();
            emit(new Event("model_attempt", step, Integer.toString(attempt)));
            try {
                return model.complete(request, cancellation);
            } catch (CancellationException exception) {
                throw exception;
            } catch (Exception exception) {
                last = exception;
                if (attempt == options.maxAttempts()) {
                    break;
                }
                long delay = options.baseDelayMillis() <= 0 ? 50 : options.baseDelayMillis() * (1L << (attempt - 1));
                sleepCancellable(delay, cancellation);
            }
        }
        throw last == null ? new IllegalStateException("model failed") : last;
    }

    private List<ToolResult> executeBatch(Message assistant, CancellationToken cancellation, int step) throws Exception {
        if (options.executionMode() == ExecutionMode.SEQUENTIAL || assistant.toolCalls().size() < 2) {
            List<ToolResult> results = new ArrayList<>();
            for (ToolCall call : assistant.toolCalls()) {
                results.add(executeOne(call, cancellation, step));
            }
            return results;
        }
        ExecutorService executor = Executors.newFixedThreadPool(assistant.toolCalls().size());
        try {
            List<Future<ToolResult>> futures = new ArrayList<>();
            for (ToolCall call : assistant.toolCalls()) {
                futures.add(executor.submit(() -> executeOne(call, cancellation, step)));
            }
            List<ToolResult> results = new ArrayList<>();
            for (Future<ToolResult> future : futures) {
                try {
                    results.add(future.get());
                } catch (ExecutionException exception) {
                    Throwable cause = exception.getCause();
                    if (cause instanceof Exception checked) {
                        throw checked;
                    }
                    throw exception;
                }
            }
            return results;
        } finally {
            executor.shutdownNow();
        }
    }

    private ToolResult executeOne(ToolCall call, CancellationToken cancellation, int step) {
        emit(new Event("tool_start", step, call.name()));
        Tool tool;
        try {
            tool = registry.validate(call);
            if (options.beforeTool() != null) {
                options.beforeTool().check(cancellation, call, tool.definition());
            }
            cancellation.throwIfCancelled();
            ToolResult result = tool.execute(cancellation, call.arguments());
            if (options.afterTool() != null) {
                result = options.afterTool().apply(cancellation, call, result);
            }
            emit(new Event("tool_end", step, call.name()));
            return result;
        } catch (CancellationException exception) {
            throw exception;
        } catch (Exception exception) {
            ToolResult result = ToolResult.error(exception.getMessage() == null ? exception.toString() : exception.getMessage());
            emit(new Event("tool_error", step, call.name() + ": " + result.content()));
            return result;
        }
    }

    private void emit(Event event) {
        options.events().accept(event);
    }

    private static void sleepCancellable(long millis, CancellationToken cancellation) {
        long deadline = System.currentTimeMillis() + millis;
        while (System.currentTimeMillis() < deadline) {
            cancellation.throwIfCancelled();
            try {
                Thread.sleep(Math.min(10, Math.max(1, deadline - System.currentTimeMillis())));
            } catch (InterruptedException exception) {
                Thread.currentThread().interrupt();
                throw new CancellationException("operation interrupted");
            }
        }
    }
}
