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
 * а живучесть опроса - дело Poller. Единственная задача класса - выполнить запрос и
 * внятно упасть, если не вышло.
 */
public final class Telegram {
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
            throw new IllegalStateException("Telegram ответил отказом: " + res.body());
        }
        return json;
    }

    public void sendMessage(String text) throws Exception {
        JsonObject b = new JsonObject();
        b.addProperty("chat_id", cfg.chatId);
        if (cfg.threadId > 0) b.addProperty("message_thread_id", cfg.threadId);
        b.addProperty("text", text);
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
}
