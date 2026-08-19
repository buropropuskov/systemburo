<template>
  <div
    class="notifications"
    data-testid="cabinet-notifications"
  >
    <div class="notifications__header">
      <div class="notifications__header-left">
        <h3 class="notifications__title">
          Уведомления
        </h3>
        <span
          v-if="unreadCount > 0"
          class="notification-badge"
        >{{ unreadCount }}</span>
        <NotificationFilterTabs
          :model-value="filter"
          @update:model-value="setFilter"
        />
      </div>
      <div class="notifications__header-actions">
        <button
          type="button"
          class="notifications__settings"
          data-testid="cabinet-notifications-settings"
          title="Настроить уведомления"
          aria-label="Настроить уведомления"
          @click="openSettings"
        >
          <AppIcon
            name="settings"
            :size="19"
            class="notifications__settings-icon"
          />
        </button>
        <button
          v-if="notifications.length > 0"
          type="button"
          class="notifications__clear"
          data-testid="cabinet-notifications-clear"
          @click="clearAll"
        >
          Очистить
        </button>
      </div>
    </div>

    <div
      ref="listScroll"
      class="notifications__list"
      :class="{ 'empty-list': notifications.length === 0 && !listLoading }"
    >
      <div
        v-if="listLoading && notifications.length === 0"
        class="notifications__loading"
      >
        <LoaderSpinner />
      </div>
      <div
        v-else-if="notifications.length === 0"
        class="notifications__empty"
      >
        <p>Уведомлений нет</p>
      </div>
      <div
        v-else
        class="notifications__items"
      >
        <template
          v-for="notif in notifications"
          :key="notif.id"
        >
          <div
            class="notification-item"
            :class="{ unread: !notif.is_read }"
          >
            <div class="notification-dot-wrapper">
              <transition name="dot-fade">
                <div
                  v-if="!notif.is_read"
                  class="notification-dot"
                />
              </transition>
            </div>
            <div
              class="notification-content"
              @click="markRead(notif)"
            >
              <div class="notification-header">
                <div class="notification-title">
                  {{ notif.title }}
                </div>
                <span
                  v-if="(notif.count || 1) > 1"
                  class="notification-count"
                  :title="`Событий: ${notif.count}`"
                >{{ notif.count }}</span>
                <div class="notification-date">
                  {{ relativeTime(notif.created_at) }}
                </div>
              </div>
              <div class="notification-message">
                {{ notif.message }}
              </div>
            </div>
            <button
              type="button"
              class="delete-btn"
              :aria-label="`Удалить уведомление ${notif.title || ''}`"
              @click.stop="deleteNotification(notif.id)"
            >
              <svg
                width="10"
                height="10"
                viewBox="0 0 14 14"
                fill="none"
                aria-hidden="true"
              >
                <path
                  d="M13 1L1 13M1 1L13 13"
                  stroke="#a2a2a2"
                  stroke-width="2"
                  stroke-linecap="round"
                />
              </svg>
            </button>
          </div>
        </template>

        <!-- Бесшовная подгрузка (#1748 S7, зеркало #1158/#1173). -->
        <div
          v-if="hasMoreNotifications"
          :ref="setSentinelRef"
          class="notifications__sentinel"
        >
          <div
            v-if="listLoading"
            class="notifications__spinner"
          />
          <div
            v-else-if="listError"
            class="sentinel-error"
          >
            <span>Не удалось загрузить ещё</span>
            <button
              type="button"
              class="lk-button lk-button--secondary lk-button--sm"
              :disabled="listLoading"
              @click="retryNotificationsList"
            >
              Повторить
            </button>
          </div>
        </div>
      </div>
    </div>

    <NotificationDetailModal
      :show="showDetailModal"
      :notification="detailNotification"
      @close="showDetailModal = false"
      @action="handleDetailAction"
      @unread="handleDetailUnread"
      @delete="handleDetailDelete"
    />
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import { getNotificationsPaginated } from '@/api/notifications'
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue'
import { useUiStore } from '@/stores/ui'
import eventStream from '@/services/eventStream'
import { useNotificationNavigation } from '@/composables/useNotificationNavigation'
import { useInfiniteList } from '@/composables/useInfiniteList'
import { formatTimeAgo } from '@/utils/datetime'
import { parseNotificationData } from '@/utils/notificationDetails'
import NotificationDetailModal from '@/components/notifications/NotificationDetailModal.vue'
import NotificationFilterTabs from '@/components/notifications/NotificationFilterTabs.vue'
import AppIcon from '@/components/icons/AppIcon.vue'

// Блок в личном кабинете компактнее колокольчика (min/max-height 200px) - та же логика
// размера страницы, что и в UserNotifications.vue (см. её комментарий).
const NOTIFICATIONS_PER_PAGE = 20

export default {
  name: 'UserNotificationsInline',

  components: { LoaderSpinner, NotificationDetailModal, NotificationFilterTabs, AppIcon },

  setup() {
    const { resolveApplicationRoute } = useNotificationNavigation();
    // useInfiniteList (#1158/#1748 S7): аккумуляция страниц, hasMore/canLoadMore,
    // устойчивость к ошибкам бэка (#1173).
    const infiniteList = useInfiniteList({ perPage: NOTIFICATIONS_PER_PAGE });
    return {
      resolveApplicationRoute,
      notifications: infiniteList.items,
      hasMoreNotifications: infiniteList.hasMore,
      listLoading: infiniteList.loading,
      listError: infiniteList.error,
      loadNotificationsList: infiniteList.load,
      loadMoreNotificationsList: infiniteList.loadMore,
      retryNotificationsList: infiniteList.retry,
      observeNotificationsSentinel: infiniteList.observeSentinel,
      disconnectNotificationsSentinel: infiniteList.disconnectObserver,
    };
  },

  data() {
    return {
      filter: 'all',
      unreadCount: 0,
      pollTimer: null,
      eventStreamOff: null,
      eventStreamStatusOff: null,
      sseConnected: false,
      showDetailModal: false,
      detailNotification: null,
    }
  },

  computed: {
    // Разделители "Сегодня"/"Вчера"/дата (#1748 S7) - сегментирует УЖЕ отсортированную
    // бэком ленту, порядок не меняет.
  },

  mounted() {
    this.fetchNotifications()
    this.pollTimer = setInterval(() => {
      // На живом SSE поллинг молчит (сигнал уже обновил список) - таймер
      // продолжает крутиться и мгновенно подхватывает при разрыве соединения.
      if (this.sseConnected) return
      this.fetchNotifications()
    }, 30000)

    // Real-time доставка (#840 V1): по сигналу сервера мгновенно перезапрашиваем
    // уведомления вместо ожидания 30с-поллинга.
    eventStream.connect()
    this.eventStreamOff = eventStream.subscribe('notifications', () => {
      this.fetchNotifications()
    })
    this.eventStreamStatusOff = eventStream.onStatus((status) => {
      this.sseConnected = status === 'connected'
    })
  },

  beforeUnmount() {
    this.disconnectNotificationsSentinel()
    if (this.pollTimer) {
      clearInterval(this.pollTimer)
      this.pollTimer = null
    }
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
    // fetchPage для useInfiniteList (#1748 S7): limit/offset + filter текущей вкладки.
    // meta.unread_count независим от страницы/фильтра - обновляет счётчик в шапке
    // на КАЖДОМ ответе, не только на первой странице.
    async buildNotificationsPage(page, perPage) {
      const offset = (page - 1) * perPage
      const { items, total, unreadCount } = await getNotificationsPaginated({
        limit: perPage,
        offset,
        filter: this.filter,
      })
      this.unreadCount = unreadCount
      return { items, total }
    },

    async fetchNotifications() {
      try {
        await this.loadNotificationsList(this.buildNotificationsPage, { reset: true })
      } catch {
        // ошибка первичной загрузки уже отражена в listError - background poll/
        // real-time не должен кидать тост
      }
    },

    setFilter(value) {
      if (this.filter === value) return
      this.filter = value
      this.fetchNotifications()
    },

    // Автодогрузка следующей порции по пересечению sentinel со списком (#1158/#1748 S7).
    setSentinelRef(el) {
      this.observeNotificationsSentinel(el, this.buildNotificationsPage, { root: this.$refs.listScroll || null })
    },

    async markRead(notif) {
      if (!notif.is_read) {
        try {
          const response = await apiRequest(`/notifications/${notif.id}/read`, {
            method: 'PUT',
            body: JSON.stringify({ is_read: true }),
          })
          if (response.ok) {
            notif.is_read = true
            this.unreadCount = Math.max(0, this.unreadCount - 1)
          }
        } catch {
          // ignore
        }
      }
      // Клик по карточке раскрывает подробности в модалке (#1748 S6) - переход
      // к заявке теперь делает кнопка действия внутри неё, не сам клик.
      this.detailNotification = notif
      this.showDetailModal = true
    },

    // Кнопка действия модалки — то же, что раньше делал клик по карточке о заявке (#973).
    // application_id для навигации лежит в data (jsonb-строка) уведомлений о заявках.
    handleDetailAction() {
      const data = parseNotificationData(this.detailNotification)
      const appId = data.application_id ? Number(data.application_id) : null
      if (!appId) return
      this.showDetailModal = false
      this.$router.push(this.resolveApplicationRoute(appId)).catch(() => {})
    },

    async handleDetailUnread() {
      const notif = this.detailNotification
      if (!notif) return
      try {
        const response = await apiRequest(`/notifications/${notif.id}/read`, {
          method: 'PUT',
          body: JSON.stringify({ is_read: false }),
        })
        if (response.ok) {
          const wasRead = notif.is_read
          notif.is_read = false
          if (wasRead) this.unreadCount += 1
        }
      } catch {
        // ignore
      }
      this.showDetailModal = false
    },

    async handleDetailDelete() {
      const notif = this.detailNotification
      if (!notif) return
      await this.deleteNotification(notif.id)
      this.showDetailModal = false
    },

    async deleteNotification(id) {
      try {
        const response = await apiRequest(`/notifications/${id}`, { method: 'DELETE' })
        if (response.ok) {
          const removed = this.notifications.find((n) => n.id === id)
          this.notifications = this.notifications.filter(n => n.id !== id)
          if (removed && !removed.is_read) {
            this.unreadCount = Math.max(0, this.unreadCount - 1)
          }
        }
      } catch {
        // ignore
      }
    },

    openSettings() {
      this.$router.push('/notification-settings').catch(() => {})
    },

    async clearAll() {
      const ok = await useUiStore().confirm({
        title: 'Очистить уведомления?',
        message: 'Все уведомления будут удалены.',
        confirmText: 'Очистить',
        cancelText: 'Отмена',
        danger: false,
      })
      if (!ok) return
      try {
        const response = await apiRequest('/notifications', { method: 'DELETE' })
        if (response.ok) {
          this.notifications = []
          this.unreadCount = 0
        }
      } catch {
        // ignore
      }
    },

    // Обёртка над общим форматтером (@/utils/datetime) - раньше здесь была своя копия
    // логики "N назад" (#1748 S7 дедуп).
    relativeTime(dateStr) {
      return formatTimeAgo(dateStr)
    },
  },
}
</script>

<style scoped>
.notifications {
  flex: 1;
  min-height: var(--cabinet-card-height, 200px);
  max-height: var(--cabinet-card-height, 200px);
  background-color: var(--surface);
  border-radius: 30px;
  border: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.notifications__header {
  min-height: 48px;
  width: 100%;
  border-bottom: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  /* flex-start, а не center: на узком экране фильтры (Все/Непрочитанные) переносятся
     на вторую строку внутри header-left, и по центру кнопки оказывались между
     строками. Прижимаем к верху - кнопки в одну линию с заголовком "Уведомления". */
  align-items: flex-start;
  padding: 10px 20px;
  flex-shrink: 0;
  background: var(--surface);
  gap: 12px;
}

.notifications__header-left {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.notifications__header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.notifications__title {
  font-size: 14px;
  font-weight: 600;
  margin: 0;
  color: var(--text);
}

.notification-badge {
  background: var(--color-danger, var(--danger));
  color: var(--fill-text);
  border-radius: 30px;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 600;
  min-width: 20px;
  text-align: center;
}

.notifications__settings-icon {
  /* 19px при общей обводке 1.7 даёт волосок в 1.3px - на мелком значке вес задаёт CSS. */
  stroke-width: 2;
}

.notifications__settings {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  padding: 0;
  color: var(--text-muted);
  cursor: pointer;
  transition: color 0.2s ease, transform 0.2s ease;
}

.notifications__settings:hover {
  color: var(--color-text);
  /* Меньше углового шага зубцов (45): на самом шаге поворот не виден. */
  transform: rotate(22.5deg);
}

.notifications__clear {
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 12px;
  cursor: pointer;
  transition: color 0.2s ease;
  font-weight: 500;
  padding: 4px 8px;
  border-radius: 20px;
  font-family: inherit;
}

.notifications__read-all:hover:not(:disabled) {
  background: var(--accent-tint);
}

.notifications__clear:hover {
  color: var(--color-text);
  background: var(--surface-2);
}

/* Разделитель дня (#1748 S7) - "Сегодня"/"Вчера"/дата. */
.notifications__list {
  flex: 1;
  overflow-y: auto;
  padding: 0;
  max-height: 200px;
}

.notifications__list.empty-list {
  display: flex;
  align-items: center;
  justify-content: center;
}

.notifications__list::-webkit-scrollbar {
  width: 4px;
}
.notifications__list::-webkit-scrollbar-thumb {
  background: color-mix(in srgb, var(--accent) 22%, var(--surface));
  border-radius: 2px;
}

.notifications__items {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.notification-item {
  display: flex;
  align-items: flex-start;
  padding: 10px 20px;
  gap: 10px;
  border-bottom: 1px solid var(--surface-2);
  transition: background-color 0.15s;
}

.notification-item:last-child {
  border-bottom: none;
}

.notification-item.unread {
  background: color-mix(in srgb, var(--accent) 8%, var(--surface));
}

.notification-dot-wrapper {
  width: 10px;
  display: flex;
  justify-content: center;
  padding-top: 6px;
  flex-shrink: 0;
  overflow: hidden;
  /* Плавное схлопывание при прочтении: ширина -> 0 и отрицательный margin
     поглощает flex-gap, поэтому контент уезжает влево анимированно, без рывка. */
  transition: width 0.25s ease, margin-right 0.25s ease, opacity 0.2s ease;
}

.notification-item:not(.unread) .notification-dot-wrapper {
  width: 0;
  margin-right: -10px;
  opacity: 0;
}

.notification-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--color-primary);
}

.dot-fade-enter-active {
  transition: opacity 0.2s ease;
}

.dot-fade-leave-active {
  transition: opacity 0.25s ease, transform 0.25s ease;
}

.dot-fade-enter-from {
  opacity: 0;
}

.dot-fade-leave-to {
  opacity: 0;
  transform: translateX(-16px);
}

.notification-item__category {
  margin-top: 5px;
  flex-shrink: 0;
}

.notification-content {
  flex: 1;
  min-width: 0;
  cursor: pointer;
}

.notification-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 8px;
}

.notification-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
  min-width: 0;
}

/* Счётчик схлопнутых повторов (#1748 S7, count>1). */
.notification-count {
  flex-shrink: 0;
  min-width: 16px;
  height: 16px;
  padding: 0 5px;
  border-radius: 999px;
  background: var(--accent);
  color: var(--accent-contrast);
  font-size: 10px;
  font-weight: 700;
  line-height: 16px;
  text-align: center;
}

.notification-date {
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
  flex-shrink: 0;
}

.notification-message {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.delete-btn {
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  cursor: pointer;
  border-radius: 50%;
  transition: all 0.15s;
}

.delete-btn:hover {
  background: var(--danger-bg);
}

.delete-btn:hover svg path {
  stroke: var(--color-danger, var(--danger-text));
}

.notifications__loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
}

.notifications__empty {
  padding: 40px;
  color: var(--text-muted);
  font-size: 13px;
  text-align: center;
}

.notifications__empty p {
  margin: 0;
}

/* Sentinel бесшовной подгрузки (#1748 S7, зеркало #1158/#1173). */
.notifications__sentinel {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 24px;
  padding: 10px 0;
  flex-shrink: 0;
}

.notifications__spinner {
  width: 16px;
  height: 16px;
  border: 2px solid var(--color-border);
  border-top-color: var(--accent-text);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.sentinel-error {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--danger-text);
  font-size: 12px;
}
</style>
