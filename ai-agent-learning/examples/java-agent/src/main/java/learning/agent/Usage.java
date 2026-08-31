package learning.agent;

public record Usage(int inputTokens, int outputTokens, int totalTokens, double cost) {
    public static Usage empty() {
        return new Usage(0, 0, 0, 0);
    }
}
