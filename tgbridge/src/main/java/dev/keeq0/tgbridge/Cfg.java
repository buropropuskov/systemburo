package dev.keeq0.tgbridge;

import org.bukkit.configuration.file.FileConfiguration;

import java.util.HashMap;
import java.util.List;
import java.util.Map;

/** Снимок конфигурации. Пересоздаётся целиком при перезагрузке, поля неизменяемы. */
public final class Cfg {
    public final String token;
    public final String apiBase;
    public final long chatId;
    public final long chatThreadId;
    public final long systemThreadId;

    public final int maxAgeSeconds;
    public final int maxIncomingAgeSeconds;
    public final int maxSize;
    public final List<Integer> backoffSeconds;
    public final long batchWindowMillis;
    public final int batchMaxMessages;

    public final boolean evChat, evJoin, evQuit, evDeath, evStart, evStop, evLoginDenied;
    public final boolean cmdOnline, cmdTps, cmdMap, cmdStatus;
    public final String mapUrl;

    public final String fChat, fJoin, fQuit, fDeath, fStart, fRestart, fStop, fLoginDenied;
    public final int restartWindowSeconds;

    public final String mcFormat;
    public final boolean linkify;
    public final Map<String, String> names;

    public final boolean warnTps;
    public final double warnTpsBelow;
    public final int warnTpsForSeconds, warnTpsCooldownMinutes;
    public final String fWarnTps;

    public final boolean warnDisk;
    public final double warnDiskFreeGb;
    public final int warnDiskEveryMinutes;
    public final String fWarnDisk;

    public Cfg(FileConfiguration c) {
        token = c.getString("bot.token", "").trim();
        apiBase = c.getString("bot.apiBase", "https://api.telegram.org").replaceAll("/+$", "");
        chatId = c.getLong("bot.chatId", 0L);
        chatThreadId = c.getLong("bot.threadId", -1L);
        // служебный топик необязателен: по умолчанию всё идёт в общий
        long sys = c.getLong("bot.systemThreadId", -1L);
        systemThreadId = sys > 0 ? sys : chatThreadId;

        maxAgeSeconds = c.getInt("queue.maxAgeSeconds", 600);
        maxIncomingAgeSeconds = c.getInt("queue.maxIncomingAgeSeconds", 120);
        maxSize = Math.max(10, c.getInt("queue.maxSize", 200));
        List<Integer> b = c.getIntegerList("queue.backoffSeconds");
        backoffSeconds = b.isEmpty() ? List.of(2, 5, 10, 30, 60) : List.copyOf(b);
        batchWindowMillis = c.getLong("queue.batchWindowMillis", 700L);
        batchMaxMessages = Math.max(1, c.getInt("queue.batchMaxMessages", 8));

        evChat = c.getBoolean("events.chat", true);
        evJoin = c.getBoolean("events.join", true);
        evQuit = c.getBoolean("events.quit", true);
        evDeath = c.getBoolean("events.death", true);
        evStart = c.getBoolean("events.serverStart", true);
        evStop = c.getBoolean("events.serverStop", true);
        evLoginDenied = c.getBoolean("events.loginDenied", true);

        cmdOnline = c.getBoolean("commands.online", true);
        cmdTps = c.getBoolean("commands.tps", true);
        cmdMap = c.getBoolean("commands.map", true);
        cmdStatus = c.getBoolean("commands.status", true);
        mapUrl = c.getString("commands.mapUrl", "");

        fChat = c.getString("telegramFormats.chat", "[MC] {name}: {message}");
        fJoin = c.getString("telegramFormats.join", "{name} зашёл на сервер");
        fQuit = c.getString("telegramFormats.quit", "{name} вышел с сервера");
        fDeath = c.getString("telegramFormats.death", "{message}");
        fStart = c.getString("telegramFormats.serverStart", "Сервер включен.");
        fRestart = c.getString("telegramFormats.serverRestart", "Сервер перезагружен.");
        fStop = c.getString("telegramFormats.serverStop", "Сервер выключен.");
        fLoginDenied = c.getString("telegramFormats.loginDenied", "Отклонён вход: {name} ({reason})");
        restartWindowSeconds = c.getInt("telegramFormats.restartWindowSeconds", 300);

        mcFormat = c.getString("minecraftFormat", "<blue>[Telegram]</blue> <white>{name}</white>: {message}");
        linkify = c.getBoolean("clickableLinks", true);

        names = new HashMap<>();
        var section = c.getConfigurationSection("names");
        if (section != null) {
            for (String key : section.getKeys(false)) names.put(key.toLowerCase(), section.getString(key, key));
        }

        warnTps = c.getBoolean("alerts.tps.enabled", true);
        warnTpsBelow = c.getDouble("alerts.tps.below", 15.0);
        warnTpsForSeconds = c.getInt("alerts.tps.forSeconds", 30);
        warnTpsCooldownMinutes = c.getInt("alerts.tps.cooldownMinutes", 15);
        fWarnTps = c.getString("alerts.tps.message", "Сервер тормозит: TPS {tps}, тик {mspt} мс");

        warnDisk = c.getBoolean("alerts.disk.enabled", true);
        warnDiskFreeGb = c.getDouble("alerts.disk.freeGb", 2.0);
        warnDiskEveryMinutes = c.getInt("alerts.disk.everyMinutes", 60);
        fWarnDisk = c.getString("alerts.disk.message", "Мало места на диске: свободно {free} ГБ");
    }

    public boolean usable() {
        return !token.isEmpty() && chatId != 0L;
    }

    /** Понятное имя вместо юзернейма, если задано в конфиге. */
    public String displayName(String telegramName) {
        return names.getOrDefault(telegramName.toLowerCase(), telegramName);
    }
}
