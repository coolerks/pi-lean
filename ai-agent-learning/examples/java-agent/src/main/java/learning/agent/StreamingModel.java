package learning.agent;

import java.util.function.Consumer;

public interface StreamingModel {
    Response stream(Request request, CancellationToken cancellation, Consumer<String> onDelta) throws Exception;
}
