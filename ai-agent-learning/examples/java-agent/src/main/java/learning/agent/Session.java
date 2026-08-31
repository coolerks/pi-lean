package learning.agent;

import com.fasterxml.jackson.databind.ObjectMapper;

import java.io.BufferedReader;
import java.io.BufferedWriter;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.util.ArrayList;
import java.util.List;

public final class Session {
    private final ObjectMapper mapper;
    private final List<Message> messages = new ArrayList<>();

    public Session(ObjectMapper mapper) {
        this.mapper = mapper;
    }

    public synchronized void append(Message... entries) {
        messages.addAll(List.of(entries));
    }

    public synchronized List<Message> snapshot() {
        return List.copyOf(messages);
    }

    public synchronized void replace(List<Message> entries) {
        messages.clear();
        messages.addAll(entries);
    }

    public synchronized void save(Path path) throws IOException {
        Path directory = path.getParent() == null ? Path.of(".") : path.getParent();
        Files.createDirectories(directory);
        Path temporary = Files.createTempFile(directory, ".session-", ".tmp");
        try {
            try (BufferedWriter writer = Files.newBufferedWriter(temporary, StandardCharsets.UTF_8)) {
                for (Message message : messages) {
                    writer.write(mapper.writeValueAsString(message));
                    writer.newLine();
                }
            }
            try {
                Files.move(temporary, path, StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE);
            } catch (java.nio.file.AtomicMoveNotSupportedException exception) {
                Files.move(temporary, path, StandardCopyOption.REPLACE_EXISTING);
            }
        } finally {
            Files.deleteIfExists(temporary);
        }
    }

    public static Session load(Path path, ObjectMapper mapper) throws IOException {
        Session session = new Session(mapper);
        if (!Files.exists(path)) {
            return session;
        }
        try (BufferedReader reader = Files.newBufferedReader(path, StandardCharsets.UTF_8)) {
            String line;
            int number = 0;
            while ((line = reader.readLine()) != null) {
                number++;
                if (line.isBlank()) {
                    continue;
                }
                try {
                    session.append(mapper.readValue(line, Message.class));
                } catch (RuntimeException | IOException exception) {
                    throw new IOException("invalid session line " + number, exception);
                }
            }
        }
        return session;
    }
}
