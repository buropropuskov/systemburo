package dev.keeq0.tgbridge;

import net.kyori.adventure.text.Component;
import net.kyori.adventure.text.TextReplacementConfig;
import net.kyori.adventure.text.event.ClickEvent;
import net.kyori.adventure.text.format.TextDecoration;
import net.kyori.adventure.text.minimessage.MiniMessage;
import net.kyori.adventure.text.minimessage.tag.resolver.Placeholder;
import org.bukkit.Bukkit;
import org.bukkit.command.Command;
import org.bukkit.command.CommandSender;
import org.bukkit.entity.Player;
import org.bukkit.plugin.java.JavaPlugin;
import org.bukkit.scheduler.BukkitTask;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.regex.Pattern;
import java.util.stream.Collectors;

public final class TgBridge extends JavaPlugin {
    private static final Pattern URL = Pattern.compile("https?://\\S+");

    private volatile Cfg cfg;
    private Telegram telegram;
    private OutboundQueue queue;
    private Poller poller;
    private Path stampFile;
    private final long startedAt = System.currentTimeMillis();

    private BukkitTask tpsWatch;
    private BukkitTask diskWatch;
    private long lowTpsSince = 0;
    private long lastTpsWarn = 0;

    @Override
    public void onEnable() {
        saveDefaultConfig();
        stampFile = getDataFolder().toPath().resolve("last-stop");
        // слушатели вешаем до проверки конфига: иначе после /tgbridge reload мост живёт,
        // а события игры до него не доходят
        getServer().getPluginManager().registerEvents(new Listeners(this), this);
        if (!startBridge()) {
            getLogger().warning("токен или chatId не заданы, мост спит. Заполни config.yml и выполни /tgbridge reload");
            return;
        }
        announceStartup();
    }

    @Override
    public void onDisable() {
        writeStamp();
        if (queue != null && cfg != null && cfg.usable() && cfg.evStop) {
            // ждём доставки не дольше двух секунд: остановка сервера важнее прощального сообщения
            queue.sendBlocking(cfg.fStop, cfg.systemThreadId, 2000);
        }
        stopBridge();
    }

    private boolean startBridge() {
        reloadConfig();
        cfg = new Cfg(getConfig());
        if (!cfg.usable()) return false;
        telegram = new Telegram(cfg);
        queue = new OutboundQueue(cfg, telegram, getLogger(), getDataFolder().toPath().resolve("queue.json"));
        queue.start();
        poller = new Poller(cfg, telegram, getLogger(), this::onTelegramMessage);
        poller.start();
        startWatchers();
        return true;
    }

    private void stopBridge() {
        if (tpsWatch != null) tpsWatch.cancel();
        if (diskWatch != null) diskWatch.cancel();
        if (poller != null) poller.stop();
        if (queue != null) queue.stop();
    }

    /** Наблюдатели за TPS и местом на диске: на одном ядре и лимите в 20 ГБ это не роскошь. */
    private void startWatchers() {
        if (cfg.warnTps) {
            tpsWatch = getServer().getScheduler().runTaskTimer(this, () -> {
                double tps = Bukkit.getServer().getTPS()[0];
                long now = System.currentTimeMillis();
                if (tps >= cfg.warnTpsBelow) {
                    lowTpsSince = 0;
                    return;
                }
                if (lowTpsSince == 0) lowTpsSince = now;
                boolean longEnough = now - lowTpsSince >= cfg.warnTpsForSeconds * 1000L;
                boolean cooled = now - lastTpsWarn >= cfg.warnTpsCooldownMinutes * 60_000L;
                if (longEnough && cooled) {
                    lastTpsWarn = now;
                    queue.enqueue(cfg.fWarnTps
                            .replace("{tps}", String.format("%.1f", tps))
                            .replace("{mspt}", String.format("%.0f", Bukkit.getServer().getAverageTickTime())),
                            cfg.systemThreadId);
                }
            }, 200L, 200L);
        }
        if (cfg.warnDisk) {
            long period = Math.max(1, cfg.warnDiskEveryMinutes) * 60L * 20L;
            diskWatch = getServer().getScheduler().runTaskTimerAsynchronously(this, () -> {
                double freeGb = getDataFolder().getUsableSpace() / 1024.0 / 1024.0 / 1024.0;
                if (freeGb < cfg.warnDiskFreeGb) {
                    queue.enqueue(cfg.fWarnDisk.replace("{free}", String.format("%.1f", freeGb)),
                            cfg.systemThreadId);
                }
            }, 600L, period);
        }
    }

    /** Отличаем холодный старт от перезагрузки по метке времени прошлой остановки. */
    private void announceStartup() {
        if (!cfg.evStart) return;
        long stopped = readStamp();
        boolean restart = stopped > 0
                && (System.currentTimeMillis() - stopped) < cfg.restartWindowSeconds * 1000L;
        queue.enqueue(restart ? cfg.fRestart : cfg.fStart, cfg.systemThreadId);
    }

    private long readStamp() {
        try {
            if (!Files.exists(stampFile)) return 0L;
            return Long.parseLong(Files.readString(stampFile).trim());
        } catch (Exception e) {
            return 0L;
        }
    }

    private void writeStamp() {
        try {
            Files.createDirectories(stampFile.getParent());
            Files.writeString(stampFile, Long.toString(System.currentTimeMillis()));
        } catch (IOException ignored) {
        }
    }

    /** Вызывается из потока опроса, поэтому всё, что трогает сервер, уходит в основной поток. */
    private void onTelegramMessage(String name, String text) {
        String lower = text.toLowerCase();
        if (cfg.cmdOnline && (lower.startsWith("/online") || lower.startsWith("/онлайн"))) {
            getServer().getScheduler().runTask(this, () -> queue.enqueue(onlineReport()));
            return;
        }
        if (cfg.cmdTps && lower.startsWith("/tps")) {
            getServer().getScheduler().runTask(this, () -> queue.enqueue(tpsReport()));
            return;
        }
        if (cfg.cmdStatus && lower.startsWith("/status")) {
            getServer().getScheduler().runTask(this, () -> queue.enqueue(statusReport()));
            return;
        }
        if (cfg.cmdMap && lower.startsWith("/map")) {
            queue.enqueue(cfg.mapUrl.isBlank() ? "Адрес карты не задан в конфиге." : cfg.mapUrl);
            return;
        }
        if (text.startsWith("/")) return;

        Component msg = MiniMessage.miniMessage().deserialize(
                cfg.mcFormat.replace("{name}", "<tg_name>").replace("{message}", "<tg_text>"),
                Placeholder.unparsed("tg_name", cfg.displayName(name)),
                Placeholder.unparsed("tg_text", text));
        if (cfg.linkify) msg = linkify(msg);
        Component out = msg;
        getServer().getScheduler().runTask(this, () -> Bukkit.broadcast(out));
    }

    /** Ссылки из Telegram делаем кликабельными: иначе их приходится перепечатывать руками. */
    private static Component linkify(Component source) {
        return source.replaceText(TextReplacementConfig.builder()
                .match(URL)
                .replacement(match -> Component.text(match.content())
                        .clickEvent(ClickEvent.openUrl(match.content()))
                        .decorate(TextDecoration.UNDERLINED))
                .build());
    }

    private String onlineReport() {
        var players = Bukkit.getOnlinePlayers();
        if (players.isEmpty()) return "Сейчас на сервере никого нет.";
        String names = players.stream().map(Player::getName).collect(Collectors.joining(", "));
        return "Сейчас играют (" + players.size() + "): " + names;
    }

    private String tpsReport() {
        double[] tps = Bukkit.getServer().getTPS();
        return String.format("TPS: %.2f / %.2f / %.2f, тик %.1f мс, онлайн %d",
                Math.min(tps[0], 20.0), Math.min(tps[1], 20.0), Math.min(tps[2], 20.0),
                Bukkit.getServer().getAverageTickTime(), Bukkit.getOnlinePlayers().size());
    }

    private String statusReport() {
        Runtime rt = Runtime.getRuntime();
        long usedMb = (rt.totalMemory() - rt.freeMemory()) / 1024 / 1024;
        long maxMb = rt.maxMemory() / 1024 / 1024;
        double freeGb = getDataFolder().getUsableSpace() / 1024.0 / 1024.0 / 1024.0;
        Duration up = Duration.ofMillis(System.currentTimeMillis() - startedAt);
        String uptime = up.toHours() > 0
                ? up.toHours() + " ч " + up.toMinutesPart() + " мин"
                : up.toMinutes() + " мин";
        return String.format("""
                Аптайм: %s
                %s
                Память: %d из %d МБ
                Свободно на диске: %.1f ГБ""", uptime, tpsReport(), usedMb, maxMb, freeGb);
    }

    @Override
    public boolean onCommand(CommandSender sender, Command command, String label, String[] args) {
        if (!sender.hasPermission("tgbridge.admin")) {
            sender.sendMessage("Недостаточно прав.");
            return true;
        }
        if (args.length == 0 || args[0].equalsIgnoreCase("status")) {
            if (cfg == null || !cfg.usable()) {
                sender.sendMessage("TgBridge: не настроен (нет токена или chatId).");
                return true;
            }
            String out = queue.online()
                    ? (queue.sent() > 0 ? "работает" : "готова, сообщений ещё не было")
                    : "в offline";
            sender.sendMessage("TgBridge: отправка " + out
                    + ", приём " + (poller.online() ? "работает" : "восстанавливается"));
            sender.sendMessage("В очереди: " + queue.pending()
                    + ", отправлено: " + queue.sent()
                    + ", отброшено по возрасту: " + queue.dropped());
            if (!queue.online() && !queue.lastError().isEmpty()) {
                sender.sendMessage("Последняя ошибка: " + queue.lastError());
            }
            return true;
        }
        if (args[0].equalsIgnoreCase("reload")) {
            stopBridge();
            boolean ok = startBridge();
            sender.sendMessage(ok ? "TgBridge перезапущен." : "TgBridge: проверь token и chatId в config.yml");
            return true;
        }
        if (args[0].equalsIgnoreCase("test")) {
            if (cfg == null || !cfg.usable()) {
                sender.sendMessage("TgBridge: не настроен.");
                return true;
            }
            queue.enqueue("Проверка связи из консоли сервера.");
            sender.sendMessage("Сообщение поставлено в очередь, следи за топиком.");
            return true;
        }
        sender.sendMessage("Использование: /tgbridge <reload|status|test>");
        return true;
    }

    public Cfg cfg() { return cfg; }

    public OutboundQueue queue() { return queue; }
}
