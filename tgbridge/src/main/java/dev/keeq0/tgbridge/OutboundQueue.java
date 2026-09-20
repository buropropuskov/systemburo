package dev.keeq0.tgbridge;

import java.util.ArrayDeque;
import java.util.Deque;
import java.util.function.BiConsumer;
import java.util.logging.Logger;

/**
 * Очередь исходящих сообщений с отдельным потоком отправки.
 *
 * Главный поток сервера сюда только кладёт строки и сразу возвращается: ни одна операция
 * здесь не блокирует игру, даже если Telegram недоступен часами. Сообщения старше
 * maxAgeSeconds выбрасываются - после долгого обрыва в топик не должна падать стена
 * позавчерашней переписки.
 */
public final class OutboundQueue {
    private record Item(String text, long bornAt) {}

    private final Deque<Item> queue = new ArrayDeque<>();
    private final Cfg cfg;
    private final Telegram tg;
    private final Logger log;

    private Thread worker;
    private volatile boolean running;
    /** (message_id, текст) для каждого доставленного сообщения - нужно для показа реакций. */
    private volatile BiConsumer<Long, String> onDelivered = (id, text) -> {};
    private volatile boolean online = true;
    private volatile String lastError = "";
    private int dropped = 0;
    private int sent = 0;

    public OutboundQueue(Cfg cfg, Telegram tg, Logger log) {
        this.cfg = cfg;
        this.tg = tg;
        this.log = log;
    }

    public void onDelivered(BiConsumer<Long, String> handler) {
        this.onDelivered = handler;
    }

    public void start() {
        running = true;
        worker = new Thread(this::loop, "TgBridge-sender");
        worker.setDaemon(true);
        worker.start();
    }

    public void enqueue(String text) {
        if (text == null || text.isBlank()) return;
        synchronized (queue) {
            if (queue.size() >= cfg.maxSize) {
                queue.pollFirst();
                dropped++;
            }
            queue.addLast(new Item(text, System.currentTimeMillis()));
            queue.notifyAll();
        }
    }

    private void loop() {
        int attempt = 0;
        while (running) {
            Item item;
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
                item = queue.peekFirst();
            }
            if (item == null) continue;

            if (System.currentTimeMillis() - item.bornAt() > cfg.maxAgeSeconds * 1000L) {
                synchronized (queue) { queue.pollFirst(); }
                dropped++;
                continue;
            }

            try {
                long id = tg.sendMessage(item.text());
                synchronized (queue) { queue.pollFirst(); }
                if (id != 0L) onDelivered.accept(id, item.text());
                sent++;
                attempt = 0;
                if (!online) {
                    online = true;
                    log.info("связь с Telegram восстановлена");
                }
            } catch (Exception e) {
                if (online) {
                    online = false;
                    log.warning("Telegram недоступен, сообщения копятся в очереди: " + e.getMessage());
                }
                lastError = e.getMessage();
                int idx = Math.min(attempt, cfg.backoffSeconds.size() - 1);
                long pause = cfg.backoffSeconds.get(idx) * 1000L;
                attempt++;
                sleep(pause);
            }
        }
    }

    /**
     * Разовая отправка в обход очереди с жёстким лимитом ожидания.
     * Нужна на выключении сервера: сообщение «Сервер выключен» стоит попытаться доставить,
     * но задерживать остановку дольше пары секунд нельзя.
     */
    public void sendBlocking(String text, long limitMs) {
        Thread t = new Thread(() -> {
            try {
                tg.sendMessage(text);
            } catch (Exception ignored) {
                // на выключении уже некому чинить, а держать сервер ради лога незачем
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

    public void stop() {
        running = false;
        synchronized (queue) { queue.notifyAll(); }
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
