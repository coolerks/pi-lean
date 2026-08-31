package learning.agent;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;

public final class SkillDiscovery {
    private SkillDiscovery() { }

    public record Skill(String name, String description, Path path) { }

    public static List<Skill> discover(Path root) throws IOException {
        List<Skill> result = new ArrayList<>();
        try (Stream<Path> paths = Files.walk(root)) {
            for (Path path : paths.filter(candidate -> candidate.getFileName().toString().equals("SKILL.md")).toList()) {
                List<String> lines = Files.readAllLines(path);
                if (lines.size() < 3 || !lines.get(0).trim().equals("---")) {
                    throw new IOException("missing frontmatter: " + path);
                }
                String name = null;
                String description = null;
                for (int index = 1; index < lines.size() && !lines.get(index).trim().equals("---"); index++) {
                    String[] pair = lines.get(index).split(":", 2);
                    if (pair.length != 2) continue;
                    if (pair[0].trim().equals("name")) name = strip(pair[1]);
                    if (pair[0].trim().equals("description")) description = strip(pair[1]);
                }
                if (name == null || description == null) {
                    throw new IOException("frontmatter requires name and description: " + path);
                }
                result.add(new Skill(name, description, path));
            }
        }
        return result;
    }

    private static String strip(String value) {
        return value.trim().replaceAll("^[\\\"']|[\\\"']$", "");
    }
}
