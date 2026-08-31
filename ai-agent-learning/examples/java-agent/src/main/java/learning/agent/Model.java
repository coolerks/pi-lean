package learning.agent;

public interface Model {
    Response complete(Request request, CancellationToken cancellation) throws Exception;
}
