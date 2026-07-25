<template>
  <!-- Мобилка: затемняющая подложка bottom-sheet (клик = закрыть). Отдельный teleport,
       чтобы панель ниже осталась прямым корнем своей transition (десктоп-дропдаун 1:1). -->
  <teleport
    to="body"
    :disabled="!isSheet"
  >
    <transition name="notif-backdrop">
      <div
        v-if="show && isSheet"
        class="notifications-backdrop"
        @click="$emit('close')"
      />
    </transition>
  </teleport>
  <teleport
    to="body"
    :disabled="!isSheet"
  >
    <transition :name="isSheet ? 'notif-sheet' : 'panel-slide'">
      <div
        v-if="show"
        class="notifications"
        :class="{ 'notifications--sheet': isSheet, 'is-dragging': sheetDragging }"
        :style="isSheet && sheetOffset ? { transform: `translateY(${sheetOffset}px)` } : null"
        @click.stop
        @touchstart="onSheetTouchStart"
        @touchmove="onSheetTouchMove"
        @touchend="onSheetTouchEnd"
      >
        <div
          v-if="isSheet"
          class="sheet-handle"
          aria-hidden="true"
        />
        <header class="notifications__header">
          <h3 class="notifications__title">
            Уведомления
            <span
              v-if="unreadCount > 0"
              class="notifications__unread-count"
            >({{ unreadCount }})</span>
          </h3>
          <button
            v-if="notifications.length > 0"
            class="notifications__clear-btn"
            @click="clearAll"
          >
            Очистить
          </button>
        </header>

        <div
          v-if="loading && notifications.length === 0"
          class="notifications__loading"
        >
          <div class="notifications__spinner" />
        </div>

        <div
          v-else-if="notifications.length === 0"
          class="notifications__empty"
        >
          <p class="notifications__empty-text">
            Нет уведомлений
          </p>
        </div>

        <ul
          v-else
          ref="sheetScroll"
          class="notifications__list"
          role="list"
        >
          <li
            v-for="item in notifications"
            :key="item.id"
            class="notification-item"
            :class="{ 'notification-item--unread': !item.is_read }"
            role="listitem"
            @click="markAsRead(item)"
          >
            <div class="notification-item__content">
              <div class="notification-item__top">
                <p
                  v-if="item.title"
                  class="notification-item__title"
                >
                  {{ item.title }}
                </p>
                <time class="notification-item__time">{{ timeAgo(item.created_at) }}</time>
              </div>
              <p
                v-if="item.message"
                class="notification-item__message"
              >
                {{ item.message }}
              </p>
            </div>
            <button
              class="notification-item__delete"
              aria-label="Удалить уведомление"
              @click.stop="deleteNotification(item.id)"
            >
              &times;
            </button>
          </li>
        </ul>
      </div>
    </transition>
  </teleport>
</template>

<script>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { apiRequest } from '@/api/client'
import { usePermissionsStore } from '@/stores/permissions'
import eventStream from '@/services/eventStream'
import { useSwipeDismiss } from '@/composables/useSwipeDismiss'

export default {
  name: 'UserNotifications',
  props: {
    show: {
      type: Boolean,
      default: false,
    },
  },
  emits: ['update:unread-count', 'close'],
  setup(props, { emit }) {
    const sheetScroll = ref(null);
    const isSheet = ref(false);
    let mql = null;
    const onChange = (e) => { isSheet.value = e.matches; };
    onMounted(() => {
      if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
      mql = window.matchMedia('(max-width: 768px)');
      isSheet.value = mql.matches;
      if (mql.addEventListener) mql.addEventListener('change', onChange);
      else if (mql.addListener) mql.addListener(onChange);
    });
    onBeforeUnmount(() => {
      if (!mql) return;
      if (mql.removeEventListener) mql.removeEventListener('change', onChange);
      else if (mql.removeListener) mql.removeListener(onChange);
    });
    const swipe = useSwipeDismiss(() => emit('close'), {
      getScrollTop: () => sheetScroll.value?.scrollTop ?? 0,
      handleSelector: '.sheet-handle',
    });
    return {
      sheetScroll,
      isSheet,
      sheetOffset: swipe.offset,
      sheetDragging: swipe.isDragging,
      onSheetTouchStart: swipe.onTouchStart,
      onSheetTouchMove: swipe.onTouchMove,
      onSheetTouchEnd: swipe.onTouchEnd,
    };
  },
  data() {
    return {
      notifications: [],
      loading: false,
      pollTimer: null,
      eventStreamOff: null,
      eventStreamStatusOff: null,
      sseConnected: false,
    }
  },
  computed: {
    unreadCount() {
      return this.notifications.filter(n => !n.is_read).length
    },
  },
  watch: {
    show(val) {
      if (val) {
        this.fetchNotifications()
      }
    },
    unreadCount(count) {
      this.$emit('update:unread-count', count)
    },
  },
  mounted() {
    // Загружаем уведомления и стартуем polling сразу - чтобы счётчик
    // непрочитанных в шапке был актуален без открытия dropdown.
    this.fetchNotifications()
    this.startPolling()

    // Real-time доставка (#840 V1): по сигналу сервера мгновенно перезапрашиваем
    // уведомления вместо ожидания 30с-поллинга. Поллинг остаётся подстраховкой -
    // гасится только пока SSE реально подключён (см. startPolling).
    eventStream.connect()
    this.eventStreamOff = eventStream.subscribe('notifications', () => {
      this.fetchNotifications()
    })
    this.eventStreamStatusOff = eventStream.onStatus((status) => {
      this.sseConnected = status === 'connected'
    })

    // Escape закрывает панель (в т.ч. мобильный bottom-sheet - конвенция ui-modals).
    this.escHandler = (e) => {
      if (e.key === 'Escape' && this.show) this.$emit('close')
    }
    document.addEventListener('keydown', this.escHandler)
  },
  beforeUnmount() {
    if (this.escHandler) document.removeEventListener('keydown', this.escHandler)
    this.stopPolling()
    if (this.eventStreamOff) {
      this.eventStreamOff()
      this.eventStreamOff = null
    }
    if (this.eventStreamStatusOff) {
      this.eventStreamStatusOff()
      this.eventStreamStatusOff = null
    }
    eventStream.disconnect()
  },
  methods: {
    async fetchNotifications() {
      this.loading = true
      try {
        const response = await apiRequest('/notifications')
        if (response.ok) {
          this.notifications = await response.json() || []
        }
      } catch {
        // background poll — не показываем ошибку
      } finally {
        this.loading = false
      }
    },

    async markAsRead(item) {
      if (!item.is_read) {
        try {
          const response = await apiRequest(`/notifications/${item.id}/read`, {
            method: 'PUT',
            body: JSON.stringify({ is_read: true }),
          })
          if (response.ok) {
            item.is_read = true
          }
        } catch {
          // ignore
        }
      }
      // Клик по уведомлению о заявке открывает её в Центре (#973).
      const appId = this.notificationAppId(item)
      if (appId) {
        this.$emit('close')
        // Заявитель без доступа к Центру заявок открывает заявку в личном кабинете (#973).
        const path = usePermissionsStore().hasPermission('page.center') ? '/center' : '/personal-cabinet'
        this.$router.push({ path, query: { open: appId } }).catch(() => {})
      }
    },

    // application_id для навигации лежит в data (jsonb-строка) уведомлений о заявках.
    notificationAppId(item) {
      let data = item.data
      if (typeof data === 'string') {
        try {
          data = JSON.parse(data)
        } catch {
          return null
        }
      }
      return data && data.application_id ? Number(data.application_id) : null
    },

    async deleteNotification(id) {
      try {
        const response = await apiRequest(`/notifications/${id}`, {
          method: 'DELETE',
        })
        if (response.ok) {
          this.notifications = this.notifications.filter(n => n.id !== id)
        }
      } catch {
        // ignore
      }
    },

    async clearAll() {
      try {
        const response = await apiRequest('/notifications', {
          method: 'DELETE',
        })
        if (response.ok) {
          this.notifications = []
          this.$emit('update:unread-count', 0)
        }
      } catch {
        // ignore
      }
    },

    startPolling() {
      this.stopPolling()
      this.pollTimer = setInterval(() => {
        // На живом SSE поллинг молчит (сигнал уже обновил список) - таймер
        // продолжает крутиться и мгновенно подхватывает при разрыве соединения.
        if (this.sseConnected) return
        this.fetchNotifications()
      }, 30000)
    },

    stopPolling() {
      if (this.pollTimer) {
        clearInterval(this.pollTimer)
        this.pollTimer = null
      }
    },

    timeAgo(dateStr) {
      if (!dateStr) return ''
      const diff = Date.now() - new Date(dateStr).getTime()
      const mins = Math.floor(diff / 60000)
      if (mins < 1) return 'только что'
      if (mins < 60) return `${mins} мин назад`
      const hours = Math.floor(mins / 60)
      if (hours < 24) return `${hours} ч назад`
      const days = Math.floor(hours / 24)
      if (days === 1) return 'вчера'
      return `${days} дн назад`
    },
  },
}
</script>

<style scoped>
.notifications {
  position: absolute;
  top: calc(100% + 10px);
  right: 0;
  width: 360px;
  background: var(--surface);
  border-radius: 20px;
  border: 1px solid var(--color-border);
  box-shadow: 0 8px 30px var(--shadow-drop);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  z-index: 200;
}

.notifications__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid var(--color-border);
  flex-shrink: 0;
}

.notifications__title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text);
}

.notifications__unread-count {
  font-weight: 400;
  color: var(--accent-text);
}

.notifications__clear-btn {
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: color 0.2s;
  font-family: 'Montserrat', sans-serif;
  padding: 0;
}

.notifications__clear-btn:hover {
  color: var(--color-text);
}

/* List */
.notifications__list {
  list-style: none;
  margin: 0;
  padding: 0;
  overflow-y: auto;
  max-height: 350px;
}

.notifications__list::-webkit-scrollbar {
  width: 4px;
}

.notifications__list::-webkit-scrollbar-track {
  background: transparent;
}

.notifications__list::-webkit-scrollbar-thumb {
  background: color-mix(in srgb, var(--accent) 22%, var(--surface));
  border-radius: 2px;
}

/* Notification item */
.notification-item {
  display: flex;
  align-items: flex-start;
  padding: 10px 16px;
  cursor: pointer;
  transition: background-color 0.15s;
  border-bottom: 1px solid var(--surface-2);
}

.notification-item:last-child {
  border-bottom: none;
}

.notification-item:hover {
  background: var(--surface-2);
}

.notification-item--unread {
  background: var(--accent-tint);
}

.notification-item--unread:hover {
  background: color-mix(in srgb, var(--accent) 18%, var(--surface));
}

.notification-item__content {
  flex: 1;
  min-width: 0;
}

.notification-item__top {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 8px;
}

.notification-item__title {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
  min-width: 0;
}

.notification-item__time {
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
  flex-shrink: 0;
}

.notification-item__message {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.notification-item__delete {
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 18px;
  cursor: pointer;
  border-radius: 50%;
  transition: all 0.15s;
  margin-left: 8px;
  margin-top: 2px;
}

.notification-item__delete:hover {
  color: var(--color-danger);
  background: var(--danger-bg);
}

/* Empty and loading states */
.notifications__loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
}

.notifications__spinner {
  width: 20px;
  height: 20px;
  border: 2px solid var(--color-border);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.notifications__empty {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
}

.notifications__empty-text {
  color: var(--text-muted);
  font-size: 13px;
}

/* Panel animation */
.panel-slide-enter-active {
  transition: all 0.2s ease-out;
}

.panel-slide-leave-active {
  transition: all 0.15s ease-in;
}

.panel-slide-enter-from,
.panel-slide-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

/* ── Мобилка: bottom-sheet (выезжает снизу вверх, свайп-закрытие). isSheet<=768 ── */
/* z-index 10000/10001: инвариант - оба оверлея (этот и .modal-overlay Объявления)
   перехватывают клики на весь экран, поэтому два bottom-sheet'а шапки одновременно
   через UI не открыть; держим ниже глобальных диалогов (ConfirmDialog 20000). */
.notifications-backdrop {
  position: fixed;
  inset: 0;
  background: var(--overlay);
  z-index: 10000;
}

.notif-backdrop-enter-active,
.notif-backdrop-leave-active {
  transition: opacity 0.3s ease;
}

.notif-backdrop-enter-from,
.notif-backdrop-leave-to {
  opacity: 0;
}

.notifications--sheet {
  position: fixed;
  top: auto;
  left: 0;
  right: 0;
  bottom: 0;
  width: 100vw;
  max-width: 100vw;
  max-height: calc(var(--app-vh, 1vh) * 90);
  border: none;
  border-radius: 16px 16px 0 0;
  box-shadow: 0 -8px 30px var(--shadow-drop);
  z-index: 10001;
  transition: transform 0.3s ease;
}

/* Во время свайпа лист следует за пальцем 1:1. */
.notifications--sheet.is-dragging {
  transition: none;
}

.notifications--sheet .sheet-handle {
  width: 40px;
  height: 4px;
  border-radius: 2px;
  background: var(--border);
  margin: 10px auto 4px;
  flex-shrink: 0;
}

/* На листе-шите список тянется на всю доступную высоту (десктопный max-height снят). */
.notifications--sheet .notifications__list {
  max-height: none;
  flex: 1;
}

.notif-sheet-enter-active,
.notif-sheet-leave-active {
  transition: transform 0.3s ease;
}

.notif-sheet-enter-from,
.notif-sheet-leave-to {
  transform: translateY(100%);
}
</style>
