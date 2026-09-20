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

    public final String inSticker, inPhoto, inVideo, inVoice, inAnimation, inDocument;
    public final boolean showReactions;
    public final String customEmojiLabel, mcReactionFormat, unknownQuote;
    public final int quoteLength;
    public final Map<String, String> emojiNames;

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

        inSticker = c.getString("incoming.sticker", "[стикер {emoji}]");
        inPhoto = c.getString("incoming.photo", "[фото]");
        inVideo = c.getString("incoming.video", "[видео]");
        inVoice = c.getString("incoming.voice", "[голосовое]");
        inAnimation = c.getString("incoming.animation", "[гиф]");
        inDocument = c.getString("incoming.document", "[файл]");

        showReactions = c.getBoolean("reactions.enabled", true);
        customEmojiLabel = c.getString("reactions.customEmojiLabel", "своим эмодзи");
        mcReactionFormat = c.getString("reactions.minecraftFormat",
                "<blue>[Telegram]</blue> <white>{name}</white> отреагировал: {emoji} на «{quote}»");
        unknownQuote = c.getString("reactions.unknownQuote", "сообщение");
        quoteLength = c.getInt("reactions.quoteLength", 40);

        emojiNames = new HashMap<>();
        var section = c.getConfigurationSection("emojiNames");
        if (section != null) {
            for (String key : section.getKeys(false)) {
                emojiNames.put(key, section.getString(key, key));
            }
        }
    }

    /** Шрифт Minecraft не рисует эмодзи, поэтому знакомые заменяем словами. */
    public String emojiLabel(String emoji) {
        return emojiNames.getOrDefault(emoji, emoji);
    }

    public boolean usable() {
        return !token.isEmpty() && chatId != 0L;
    }
}
