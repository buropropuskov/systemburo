package dev.keeq0.tgbridge;

import org.bukkit.configuration.file.FileConfiguration;

import java.util.List;

/** Снимок конфигурации. Пересоздаётся целиком при перезагрузке, поля неизменяемы. */
public final class Cfg {
    public final String token;
    public final String apiBase;
    public final long chatId;
    public final long threadId;

    public final int maxAgeSeconds;
    public final int maxIncomingAgeSeconds;
    public final int maxSize;
    public final List<Integer> backoffSeconds;

    public final boolean evChat, evJoin, evQuit, evDeath, evStart, evStop;
    public final boolean cmdOnline, cmdTps;

    public final String fChat, fJoin, fQuit, fDeath, fStart, fRestart, fStop;
    public final int restartWindowSeconds;
    public final String mcFormat;

    @SuppressWarnings("unchecked")
    public Cfg(FileConfiguration c) {
        token = c.getString("bot.token", "").trim();
        apiBase = c.getString("bot.apiBase", "https://api.telegram.org").replaceAll("/+$", "");
        chatId = c.getLong("bot.chatId", 0L);
        threadId = c.getLong("bot.threadId", -1L);

        maxAgeSeconds = c.getInt("queue.maxAgeSeconds", 600);
        maxIncomingAgeSeconds = c.getInt("queue.maxIncomingAgeSeconds", 120);
        maxSize = Math.max(10, c.getInt("queue.maxSize", 200));
        List<Integer> b = (List<Integer>) (List<?>) c.getIntegerList("queue.backoffSeconds");
        backoffSeconds = b.isEmpty() ? List.of(2, 5, 10, 30, 60) : List.copyOf(b);

        evChat = c.getBoolean("events.chat", true);
        evJoin = c.getBoolean("events.join", true);
        evQuit = c.getBoolean("events.quit", true);
        evDeath = c.getBoolean("events.death", true);
        evStart = c.getBoolean("events.serverStart", true);
        evStop = c.getBoolean("events.serverStop", true);

        cmdOnline = c.getBoolean("commands.online", true);
        cmdTps = c.getBoolean("commands.tps", true);

        fChat = c.getString("telegramFormats.chat", "[MC] {name}: {message}");
        fJoin = c.getString("telegramFormats.join", "{name} зашёл на сервер");
        fQuit = c.getString("telegramFormats.quit", "{name} вышел с сервера");
        fDeath = c.getString("telegramFormats.death", "{message}");
        fStart = c.getString("telegramFormats.serverStart", "Сервер включен.");
        fRestart = c.getString("telegramFormats.serverRestart", "Сервер перезагружен.");
        fStop = c.getString("telegramFormats.serverStop", "Сервер выключен.");
        restartWindowSeconds = c.getInt("telegramFormats.restartWindowSeconds", 300);

        mcFormat = c.getString("minecraftFormat", "<blue>[Telegram]</blue> <white>{name}</white>: {message}");
    }

    public boolean usable() {
        return !token.isEmpty() && chatId != 0L;
    }
}
