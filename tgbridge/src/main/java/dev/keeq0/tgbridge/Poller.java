package dev.keeq0.tgbridge;

import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;

import java.util.logging.Logger;

/**
 * Приём сообщений из Telegram длинным опросом.
 *
 * В отличие от готовых мостов, поток никогда не умирает от одной сетевой ошибки: он
 * логирует первую неудачу, ждёт и опрашивает снова. Именно отсутствие переподключения
 * приводило к тому, что чат замолкал до перезапуска сервера.
 */
public final class Poller {
    private final Cfg cfg;
    private final Telegram tg;
    private final Logger log;
    /** (имя отправителя, текст, id сообщения) - отфильтрованные сообщения нужного чата и топика. */
    private final Incoming onMessage;

    @FunctionalInterface
    public interface Incoming {
        void accept(String name, String text, long messageId);
    }
    /** (имя, эмодзи, id сообщения) - поставленная реакция. */
    private final Reactions onReaction;

    @FunctionalInterface
    public interface Reactions {
        void accept(String name, String emoji, long messageId);
    }

    private Thread thread;
    private volatile boolean running;
    private volatile boolean online = true;
    private long offset = 0;
    private boolean primed = false;

    public Poller(Cfg cfg, Telegram tg, Logger log,
                  Incoming onMessage, Reactions onReaction) {
        this.cfg = cfg;
        this.tg = tg;
        this.log = log;
        this.onMessage = onMessage;
        this.onReaction = onReaction;
    }

    public void start() {
        running = true;
        thread = new Thread(this::loop, "TgBridge-poller");
        thread.setDaemon(true);
        thread.start();
    }

    public void stop() {
        running = false;
        if (thread != null) thread.interrupt();
    }

    private void loop() {
        while (running) {
            try {
                if (!primed) {
                    prime();
                    continue;
                }
                JsonArray updates = tg.getUpdates(offset, 30);
                if (!online) {
                    online = true;
                    log.info("приём сообщений из Telegram восстановлен");
                }
                for (JsonElement el : updates) {
                    JsonObject upd = el.getAsJsonObject();
                    offset = upd.get("update_id").getAsLong() + 1;
                    if (upd.has("message")) {
                        handle(upd.getAsJsonObject("message"));
                    } else if (upd.has("message_reaction")) {
                        handleReaction(upd.getAsJsonObject("message_reaction"));
                    }
                }
            } catch (Exception e) {
                if (online) {
                    online = false;
                    log.warning("приём из Telegram прерван, повторяю: " + describe(e));
                }
                sleep(5000);
            }
        }
    }

    /**
     * Telegram хранит непрочитанные обновления до суток и при нулевом смещении отдаёт их
     * все разом. Без этого шага после простоя в игровой чат вываливается вчерашняя
     * переписка целиком, что однажды и случилось.
     */
    private void prime() throws Exception {
        JsonArray last = tg.getUpdates(-1, 0);
        if (!last.isEmpty()) {
            JsonObject newest = last.get(last.size() - 1).getAsJsonObject();
            offset = newest.get("update_id").getAsLong() + 1;
        }
        primed = true;
        online = true;
        log.info("подключение к Telegram установлено, старые сообщения пропущены");
    }

    private static String describe(Exception e) {
        String m = e.getMessage();
        return (m == null || m.isBlank()) ? e.getClass().getSimpleName() : m;
    }

    private void handle(JsonObject msg) {
        JsonObject chat = msg.getAsJsonObject("chat");
        if (chat == null || chat.get("id").getAsLong() != cfg.chatId) return;

        if (cfg.threadId > 0) {
            long thread = msg.has("message_thread_id") ? msg.get("message_thread_id").getAsLong() : -1;
            if (thread != cfg.threadId) return;
        }
        // защита от старых сообщений, которые Telegram мог придержать во время обрыва
        if (msg.has("date")) {
            long ageSeconds = System.currentTimeMillis() / 1000L - msg.get("date").getAsLong();
            if (ageSeconds > cfg.maxIncomingAgeSeconds) return;
        }

        String text = describeContent(msg);
        if (text == null) return;
        String name = "неизвестный";
        if (msg.has("from")) {
            JsonObject from = msg.getAsJsonObject("from");
            if (from.has("username")) name = from.get("username").getAsString();
            else if (from.has("first_name")) name = from.get("first_name").getAsString();
        }
        long id = msg.has("message_id") ? msg.get("message_id").getAsLong() : 0L;
        onMessage.accept(name, text, id);
    }

    /**
     * Текст сообщения или человекочитаемая пометка о вложении.
     * Возвращает null, если показывать нечего (например, служебное событие чата).
     */
    private String describeContent(JsonObject msg) {
        if (msg.has("text")) return msg.get("text").getAsString();

        String caption = msg.has("caption") ? " " + msg.get("caption").getAsString() : "";
        if (msg.has("sticker")) {
            JsonObject st = msg.getAsJsonObject("sticker");
            String emoji = st.has("emoji") ? st.get("emoji").getAsString() : "";
            return cfg.inSticker.replace("{emoji}", emoji).trim();
        }
        if (msg.has("photo")) return cfg.inPhoto + caption;
        if (msg.has("animation")) return cfg.inAnimation + caption;
        if (msg.has("video") || msg.has("video_note")) return cfg.inVideo + caption;
        if (msg.has("voice") || msg.has("audio")) return cfg.inVoice + caption;
        if (msg.has("document")) return cfg.inDocument + caption;
        return null;
    }

    /**
     * Реакции приходят отдельным типом обновления и не содержат ни текста сообщения,
     * ни номера топика - только его id. Поэтому наверх уходит id, а сопоставлением
     * с текстом занимается сам плагин по своему кэшу.
     */
    private void handleReaction(JsonObject r) {
        if (!cfg.showReactions) return;
        JsonObject chat = r.getAsJsonObject("chat");
        if (chat == null || chat.get("id").getAsLong() != cfg.chatId) return;
        if (!r.has("new_reaction")) return;

        JsonArray fresh = r.getAsJsonArray("new_reaction");
        if (fresh.isEmpty()) return; // реакцию сняли - молчим

        JsonObject first = fresh.get(fresh.size() - 1).getAsJsonObject();
        String emoji = first.has("emoji") ? first.get("emoji").getAsString() : "";
        if (emoji.isEmpty()) emoji = cfg.customEmojiLabel;

        String name = "неизвестный";
        if (r.has("user")) {
            JsonObject u = r.getAsJsonObject("user");
            if (u.has("username")) name = u.get("username").getAsString();
            else if (u.has("first_name")) name = u.get("first_name").getAsString();
        }
        long messageId = r.has("message_id") ? r.get("message_id").getAsLong() : 0L;
        onReaction.accept(name, emoji, messageId);
    }

    private static void sleep(long ms) {
        try {
            Thread.sleep(ms);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        }
    }

    public boolean online() { return online; }
}
