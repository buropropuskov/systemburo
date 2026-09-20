package dev.keeq0.tgbridge;

import com.google.gson.JsonArray;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayDeque;
import java.util.ArrayList;
import java.util.Deque;
import java.util.List;
import java.util.logging.Logger;

/**
 * Очередь исходящих сообщений с отдельным потоком отправки.
 *
 * Главный поток сервера сюда только кладёт строки и сразу возвращается: ни одна операция
 * здесь не блокирует игру, даже если Telegram недоступен часами.
 */
public final class OutboundQueue {
    private record Item(String text, long threadId, long bornAt) {}

    private final Deque<Item> queue = new ArrayDeque<>();
    private final Cfg cfg;
    private final Telegram tg;
    private final Logger log;
    private final Path spool;

    private Thread worker;
    private volatile boolean running;
    private volatile boolean online = true;
    private volatile String lastError = "";
    private int dropped = 0;
    private int sent = 0;

    public OutboundQueue(Cfg cfg, Telegram tg, Logger log, Path spool) {
        this.cfg = cfg;
        this.tg = tg;
        this.log = log;
        this.spool = spool;
    }

    public void start() {
        restore();
        running = true;
        worker = new Thread(this::loop, "TgBridge-sender");
        worker.setDaemon(true);
        worker.start();
    }

    public void enqueue(String text) {
        enqueue(text, cfg.chatThreadId);
    }

    public void enqueue(String text, long threadId) {
        if (text == null || text.isBlank()) return;
        synchronized (queue) {
            if (queue.size() >= cfg.maxSize) {
                queue.pollFirst();
                dropped++;
            }
            queue.addLast(new Item(text, threadId, System.currentTimeMillis()));
            queue.notifyAll();
        }
    }

    private void loop() {
        int attempt = 0;
        while (running) {
            synchronized (queue) {
                while (running && queue.isEmpty()) {
                    try {
                        queue.wait(1000);
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                        return;
                    }
                }
                if (!running) return;
            }

            // даём событиям слипнуться: перезапуск сервера иначе даёт пачку отдельных сообщений
            if (cfg.batchWindowMillis > 0) sleep(cfg.batchWindowMillis);

            List<Item> batch = takeBatch();
            if (batch.isEmpty()) continue;

            String text = join(batch);
            long threadId = batch.get(0).threadId();
            try {
                tg.sendMessage(text, threadId);
                removeSent(batch.size());
                sent += batch.size();
                attempt = 0;
                if (!online) {
                    online = true;
                    log.info("связь с Telegram восстановлена");
                }
            } catch (Telegram.RateLimited e) {
                lastError = e.getMessage();
                sleep((e.retryAfterSeconds + 1) * 1000L);
            } catch (Exception e) {
                if (online) {
                    online = false;
                    log.warning("Telegram недоступен, сообщения копятся в очереди: " + describe(e));
                }
                lastError = describe(e);
                int idx = Math.min(attempt, cfg.backoffSeconds.size() - 1);
                attempt++;
                sleep(cfg.backoffSeconds.get(idx) * 1000L);
            }
        }
    }

    /** Берём подряд идущие сообщения одного топика, пока помещаются в одно сообщение. */
    private List<Item> takeBatch() {
        List<Item> batch = new ArrayList<>();
        long now = System.currentTimeMillis();
        synchronized (queue) {
            int length = 0;
            for (Item item : queue) {
                if (now - item.bornAt() > cfg.maxAgeSeconds * 1000L) {
                    if (batch.isEmpty()) {
                        queue.pollFirst();
                        dropped++;
                        return List.of();
                    }
                    break;
                }
                if (!batch.isEmpty()) {
                    if (item.threadId() != batch.get(0).threadId()) break;
                    if (batch.size() >= cfg.batchMaxMessages) break;
                    if (length + item.text().length() + 1 > Telegram.MAX_TEXT) break;
                }
                batch.add(item);
                length += item.text().length() + 1;
            }
        }
        return batch;
    }

    private void removeSent(int count) {
        synchronized (queue) {
            for (int i = 0; i < count && !queue.isEmpty(); i++) queue.pollFirst();
        }
    }

    private static String join(List<Item> batch) {
        StringBuilder sb = new StringBuilder();
        for (Item item : batch) {
            if (sb.length() > 0) sb.append('\n');
            sb.append(item.text());
        }
        return sb.toString();
    }

    /**
     * Разовая отправка в обход очереди с жёстким лимитом ожидания.
     * Нужна на выключении: сообщение стоит попытаться доставить, но задерживать
     * остановку сервера дольше пары секунд нельзя.
     */
    public void sendBlocking(String text, long threadId, long limitMs) {
        Thread t = new Thread(() -> {
            try {
                tg.sendMessage(text, threadId);
            } catch (Exception ignored) {
                // на выключении чинить уже некому
            }
        }, "TgBridge-farewell");
        t.setDaemon(true);
        t.start();
        try {
            t.join(limitMs);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        }
    }

    /** Недоставленное переживает перезапуск сервера: иначе пропадает всё, что копилось. */
    public void persist() {
        try {
            JsonArray arr = new JsonArray();
            synchronized (queue) {
                for (Item item : queue) {
                    JsonObject o = new JsonObject();
                    o.addProperty("text", item.text());
                    o.addProperty("thread", item.threadId());
                    o.addProperty("born", item.bornAt());
                    arr.add(o);
                }
            }
            if (arr.isEmpty()) {
                Files.deleteIfExists(spool);
                return;
            }
            Files.createDirectories(spool.getParent());
            Files.writeString(spool, arr.toString());
        } catch (Exception ignored) {
        }
    }

    private void restore() {
        try {
            if (!Files.exists(spool)) return;
            JsonArray arr = JsonParser.parseString(Files.readString(spool)).getAsJsonArray();
            long now = System.currentTimeMillis();
            int restored = 0;
            synchronized (queue) {
                for (var el : arr) {
                    JsonObject o = el.getAsJsonObject();
                    long born = o.get("born").getAsLong();
                    if (now - born > cfg.maxAgeSeconds * 1000L) continue;
                    queue.addLast(new Item(o.get("text").getAsString(), o.get("thread").getAsLong(), born));
                    restored++;
                }
            }
            Files.deleteIfExists(spool);
            if (restored > 0) log.info("восстановлено недоставленных сообщений: " + restored);
        } catch (Exception ignored) {
        }
    }

    public void stop() {
        running = false;
        synchronized (queue) { queue.notifyAll(); }
        persist();
    }

    private static String describe(Exception e) {
        String m = e.getMessage();
        return (m == null || m.isBlank()) ? e.getClass().getSimpleName() : m;
    }

    private static void sleep(long ms) {
        try {
            Thread.sleep(ms);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        }
    }

    public int pending() {
        synchronized (queue) { return queue.size(); }
    }

    public boolean online() { return online; }
    public int sent() { return sent; }
    public int dropped() { return dropped; }
    public String lastError() { return lastError; }
}
