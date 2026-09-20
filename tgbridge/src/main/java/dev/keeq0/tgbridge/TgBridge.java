package dev.keeq0.tgbridge;

import net.kyori.adventure.text.Component;
import net.kyori.adventure.text.minimessage.MiniMessage;
import net.kyori.adventure.text.minimessage.tag.resolver.Placeholder;
import org.bukkit.Bukkit;
import org.bukkit.command.Command;
import org.bukkit.command.CommandSender;
import org.bukkit.entity.Player;
import org.bukkit.plugin.java.JavaPlugin;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.stream.Collectors;

public final class TgBridge extends JavaPlugin {
    private volatile Cfg cfg;
    private Telegram telegram;
    private OutboundQueue queue;
    private Poller poller;
    private Path stampFile;

    @Override
    public void onEnable() {
        saveDefaultConfig();
        stampFile = getDataFolder().toPath().resolve("last-stop");
        if (!startBridge()) {
            getLogger().warning("токен или chatId не заданы, мост спит. Заполни config.yml и выполни /tgbridge reload");
            return;
        }
        getServer().getPluginManager().registerEvents(new Listeners(this), this);
        announceStartup();
    }

    @Override
    public void onDisable() {
        writeStamp();
        if (queue != null && cfg != null && cfg.usable() && cfg.evStop) {
            // ждём доставки не дольше двух секунд: остановка сервера важнее прощального сообщения
            queue.sendBlocking(cfg.fStop, 2000);
        }
        stopBridge();
    }

    private boolean startBridge() {
        reloadConfig();
        cfg = new Cfg(getConfig());
        if (!cfg.usable()) return false;
        telegram = new Telegram(cfg);
        queue = new OutboundQueue(cfg, telegram, getLogger());
        queue.start();
        poller = new Poller(cfg, telegram, getLogger(), this::onTelegramMessage);
        poller.start();
        return true;
    }

    private void stopBridge() {
        if (poller != null) poller.stop();
        if (queue != null) queue.stop();
    }

    /** Отличаем холодный старт от перезагрузки по метке времени прошлой остановки. */
    private void announceStartup() {
        if (!cfg.evStart) return;
        long stopped = readStamp();
        boolean restart = stopped > 0
                && (System.currentTimeMillis() - stopped) < cfg.restartWindowSeconds * 1000L;
        queue.enqueue(restart ? cfg.fRestart : cfg.fStart);
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
        if (text.startsWith("/")) return;

        Component msg = MiniMessage.miniMessage().deserialize(
                cfg.mcFormat.replace("{name}", "<tg_name>").replace("{message}", "<tg_text>"),
                Placeholder.unparsed("tg_name", name),
                Placeholder.unparsed("tg_text", text));
        getServer().getScheduler().runTask(this, () -> Bukkit.broadcast(msg));
    }

    private String onlineReport() {
        var players = Bukkit.getOnlinePlayers();
        if (players.isEmpty()) return "Сейчас на сервере никого нет.";
        String names = players.stream().map(Player::getName).collect(Collectors.joining(", "));
        return "Сейчас играют (" + players.size() + "): " + names;
    }

    private String tpsReport() {
        double[] tps = Bukkit.getServer().getTPS();
        double mspt = Bukkit.getServer().getAverageTickTime();
        return String.format("TPS: %.2f / %.2f / %.2f, тик %.1f мс, онлайн %d",
                Math.min(tps[0], 20.0), Math.min(tps[1], 20.0), Math.min(tps[2], 20.0),
                mspt, Bukkit.getOnlinePlayers().size());
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
            sender.sendMessage("TgBridge: отправка " + (queue.online() ? "работает" : "в offline")
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
        sender.sendMessage("Использование: /tgbridge <reload|status>");
        return true;
    }

    public Cfg cfg() { return cfg; }

    public OutboundQueue queue() { return queue; }
}
