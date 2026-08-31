package learning.agent;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Locale;

public record PermissionPolicy(Path workspace) {
    public PermissionPolicy {
        if (workspace == null) {
            throw new IllegalArgumentException("workspace is required");
        }
        workspace = workspace.toAbsolutePath().normalize();
    }

    public Path resolvePath(String requested) throws IOException {
        Path candidate = workspace.resolve(requested == null ? "" : requested).normalize().toAbsolutePath();
        if (!candidate.startsWith(workspace)) {
            throw new SecurityException("path is outside workspace: " + requested);
        }
        return candidate;
    }

    public void checkCommand(String command) {
        if (command == null || command.isBlank()) {
            throw new IllegalArgumentException("command cannot be empty");
        }
        String lower = command.toLowerCase(Locale.ROOT);
        List<String> denied = List.of("rm -rf", "git push", "curl | sh", "wget | sh");
        for (String pattern : denied) {
            if (lower.contains(pattern)) {
                throw new SecurityException("command contains denied pattern: " + pattern);
            }
        }
    }
}
