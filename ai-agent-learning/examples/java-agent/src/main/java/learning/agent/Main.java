package learning.agent;

import com.fasterxml.jackson.databind.ObjectMapper;

import java.nio.file.Path;
import java.util.List;

public final class Main {
    private Main() { }

    public static void main(String[] args) throws Exception {
        boolean demo = java.util.Arrays.asList(args).contains("--demo");
        Path workspace = Path.of(System.getProperty("user.dir"));
        ObjectMapper mapper = new ObjectMapper();
        Model model;
        Path sessionPath;
        String prompt = "请计算 2 加 3";
        if (demo) {
            model = new ScriptedModel()
                    .add(new Response(Message.assistant(List.of(new ToolCall("call-1", "calculator",
                            mapper.readTree("{\"operation\":\"add\",\"left\":2,\"right\":3}")))), "tool_use", Usage.empty()))
                    .add(new Response(Message.assistant("2 + 3 = 5"), "stop", Usage.empty()));
            sessionPath = null;
        } else {
            String url = System.getenv("AGENT_MODEL_URL");
            String modelId = System.getenv("AGENT_MODEL");
            String key = System.getenv("AGENT_API_KEY");
            if (url == null || modelId == null || key == null) {
                System.err.println("HTTP mode requires AGENT_MODEL_URL, AGENT_MODEL, and AGENT_API_KEY; use --demo");
                System.exit(2);
                return;
            }
            model = new HttpModel(url, key, modelId);
            sessionPath = workspace.resolve(".agent-session.jsonl");
            if (args.length > 0) prompt = args[args.length - 1];
        }

        try (Harness harness = new Harness(model, workspace, sessionPath, mapper)) {
            RunResult result = harness.prompt(prompt);
            for (Message message : result.messages()) {
                if (message.role() == Message.Role.TOOL) {
                    System.out.println("tool[" + message.toolName() + "]> " + message.content());
                }
            }
            System.out.println("final> " + result.finalMessage().content());
        }
    }

}
