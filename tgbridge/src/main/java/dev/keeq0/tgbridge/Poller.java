package dev.keeq0.tgbridge;

import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;

import java.util.function.BiConsumer;
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
    /** (имя отправителя, текст) - уже отфильтрованные сообщения нужного чата и топика. */
    private final BiConsumer<String, String> onMessage;

    private Thread thread;
    private volatile boolean running;
    private volatile boolean online = true;
    private long offset = 0;
    private boolean primed = false;

    public Poller(Cfg cfg, Telegram tg, Logger log, BiConsumer<String, String> onMessage) {
        this.cfg = cfg;
        this.tg = tg;
        this.log = log;
        this.onMessage = onMessage;
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
                    if (!upd.has("message")) continue;
                    handle(upd.getAsJsonObject("message"));
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

        if (cfg.chatThreadId > 0) {
            long thread = msg.has("message_thread_id") ? msg.get("message_thread_id").getAsLong() : -1;
            if (thread != cfg.chatThreadId) return;
        }
        if (!msg.has("text")) return;

        // защита от старых сообщений, которые Telegram мог придержать во время обрыва
        if (msg.has("date")) {
            long ageSeconds = System.currentTimeMillis() / 1000L - msg.get("date").getAsLong();
            if (ageSeconds > cfg.maxIncomingAgeSeconds) return;
        }

        String text = msg.get("text").getAsString();
        String name = "неизвестный";
        if (msg.has("from")) {
            JsonObject from = msg.getAsJsonObject("from");
            if (from.has("username")) name = from.get("username").getAsString();
            else if (from.has("first_name")) name = from.get("first_name").getAsString();
        }
        onMessage.accept(name, text);
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
