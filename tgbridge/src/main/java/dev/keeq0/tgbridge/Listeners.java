package dev.keeq0.tgbridge;

import io.papermc.paper.event.player.AsyncChatEvent;
import net.kyori.adventure.text.serializer.plain.PlainTextComponentSerializer;
import org.bukkit.event.EventHandler;
import org.bukkit.event.EventPriority;
import org.bukkit.event.Listener;
import org.bukkit.event.entity.PlayerDeathEvent;
import org.bukkit.event.player.PlayerJoinEvent;
import org.bukkit.event.player.PlayerLoginEvent;
import org.bukkit.event.player.PlayerQuitEvent;

/** Игровые события -> очередь. Обработчики только форматируют строку и кладут её в очередь. */
public final class Listeners implements Listener {
    private static final PlainTextComponentSerializer PLAIN = PlainTextComponentSerializer.plainText();

    private final TgBridge plugin;

    public Listeners(TgBridge plugin) {
        this.plugin = plugin;
    }

    /** Мост может быть ещё не настроен: до заполнения config.yml события просто игнорируем. */
    private boolean asleep() {
        return plugin.cfg() == null || plugin.queue() == null;
    }

    @EventHandler(priority = EventPriority.MONITOR, ignoreCancelled = true)
    public void onChat(AsyncChatEvent e) {
        if (asleep()) return;
        Cfg cfg = plugin.cfg();
        if (!cfg.evChat) return;
        String text = PLAIN.serialize(e.message());
        plugin.queue().enqueue(format(cfg.fChat, e.getPlayer().getName(), text), cfg.chatThreadId);
    }

    @EventHandler(priority = EventPriority.MONITOR)
    public void onJoin(PlayerJoinEvent e) {
        if (asleep()) return;
        Cfg cfg = plugin.cfg();
        if (!cfg.evJoin) return;
        plugin.queue().enqueue(format(cfg.fJoin, e.getPlayer().getName(), ""), cfg.systemThreadId);
    }

    @EventHandler(priority = EventPriority.MONITOR)
    public void onQuit(PlayerQuitEvent e) {
        if (asleep()) return;
        Cfg cfg = plugin.cfg();
        if (!cfg.evQuit) return;
        plugin.queue().enqueue(format(cfg.fQuit, e.getPlayer().getName(), ""), cfg.systemThreadId);
    }

    @EventHandler(priority = EventPriority.MONITOR)
    public void onDeath(PlayerDeathEvent e) {
        if (asleep()) return;
        Cfg cfg = plugin.cfg();
        if (!cfg.evDeath || e.deathMessage() == null) return;
        String text = PLAIN.serialize(e.deathMessage());
        plugin.queue().enqueue(format(cfg.fDeath, e.getEntity().getName(), text), cfg.systemThreadId);
    }

    /**
     * Сервер работает в offline-режиме с постоянным адресом, поэтому отказы во входе
     * стоит видеть: так заметны и сканеры, и попытки зайти чужим ником.
     */
    @EventHandler(priority = EventPriority.MONITOR)
    public void onLogin(PlayerLoginEvent e) {
        if (asleep()) return;
        Cfg cfg = plugin.cfg();
        if (!cfg.evLoginDenied || e.getResult() == PlayerLoginEvent.Result.ALLOWED) return;
        String reason = switch (e.getResult()) {
            case KICK_WHITELIST -> "нет в вайтлисте";
            case KICK_BANNED -> "забанен";
            case KICK_FULL -> "сервер полон";
            default -> e.getResult().name().toLowerCase();
        };
        plugin.queue().enqueue(
                cfg.fLoginDenied.replace("{name}", e.getPlayer().getName()).replace("{reason}", reason),
                cfg.systemThreadId);
    }

    private static String format(String pattern, String name, String message) {
        return pattern.replace("{name}", name).replace("{message}", message);
    }
}
