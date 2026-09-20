package dev.keeq0.tgbridge;

import com.google.gson.JsonArray;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Duration;

/**
 * Тонкая обёртка над Bot API. Ничего не кэширует и не ретраит: повторы - дело очереди,
 * а живучесть опроса - дело Poller.
 */
public final class Telegram {
    /** Telegram попросил подождать: у групп лимит около 20 сообщений в минуту. */
    public static final class RateLimited extends Exception {
        public final int retryAfterSeconds;

        RateLimited(int seconds) {
            super("Telegram просит подождать " + seconds + " с");
            this.retryAfterSeconds = seconds;
        }
    }

    /** Предел одного сообщения в Bot API. */
    public static final int MAX_TEXT = 4096;

    private final HttpClient http;
    private final Cfg cfg;

    public Telegram(Cfg cfg) {
        this.cfg = cfg;
        this.http = HttpClient.newBuilder()
                .connectTimeout(Duration.ofSeconds(5))
                .version(HttpClient.Version.HTTP_1_1)
                .build();
    }

    private JsonObject call(String method, JsonObject body, Duration timeout) throws Exception {
        HttpRequest req = HttpRequest.newBuilder(URI.create(cfg.apiBase + "/bot" + cfg.token + "/" + method))
                .timeout(timeout)
                .header("Content-Type", "application/json; charset=utf-8")
                .POST(HttpRequest.BodyPublishers.ofString(body.toString(), StandardCharsets.UTF_8))
                .build();
        HttpResponse<String> res = http.send(req, HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8));
        JsonObject json = JsonParser.parseString(res.body()).getAsJsonObject();
        if (!json.has("ok") || !json.get("ok").getAsBoolean()) {
            if (json.has("parameters")) {
                JsonObject p = json.getAsJsonObject("parameters");
                if (p.has("retry_after")) throw new RateLimited(p.get("retry_after").getAsInt());
            }
            throw new IllegalStateException("Telegram ответил отказом: " + res.body());
        }
        return json;
    }

    public void sendMessage(String text, long threadId) throws Exception {
        JsonObject b = new JsonObject();
        b.addProperty("chat_id", cfg.chatId);
        if (threadId > 0) b.addProperty("message_thread_id", threadId);
        b.addProperty("text", trim(text));
        b.addProperty("disable_web_page_preview", true);
        call("sendMessage", b, Duration.ofSeconds(15));
    }

    /** Длинный опрос: висит на стороне Telegram до timeoutSec секунд, если сообщений нет. */
    public JsonArray getUpdates(long offset, int timeoutSec) throws Exception {
        JsonObject b = new JsonObject();
        b.addProperty("offset", offset);
        b.addProperty("timeout", timeoutSec);
        JsonArray allowed = new JsonArray();
        allowed.add("message");
        b.add("allowed_updates", allowed);
        return call("getUpdates", b, Duration.ofSeconds(timeoutSec + 15L)).getAsJsonArray("result");
    }

    static String trim(String text) {
        if (text.length() <= MAX_TEXT) return text;
        return text.substring(0, MAX_TEXT - 1) + "…";
    }
}
