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
        data-testid="ob-notifications-panel"
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
          <div class="notifications__header-top">
            <h3 class="notifications__title">
              Уведомления
              <span
                v-if="unreadCount > 0"
                class="notifications__unread-count"
              >({{ unreadCount }})</span>
            </h3>
            <div class="notifications__header-actions">
              <button
                v-if="notifications.length > 0"
                class="notifications__clear-btn"
                @click="clearAll"
              >
                Очистить
              </button>
              <button
                class="notifications__settings-btn"
                type="button"
                title="Настроить уведомления"
                aria-label="Настроить уведомления"
                data-testid="header-notifications-settings"
                @click="openSettings"
              >
                <AppIcon
                  name="settings"
                  :size="19"
                  class="notifications__settings-icon"
                />
              </button>
            </div>
          </div>
          <NotificationFilterTabs
            :model-value="filter"
            @update:model-value="setFilter"
          />
        </header>

        <div
          v-if="listLoading && notifications.length === 0"
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
          <template
            v-for="item in notifications"
            :key="item.id"
          >
            <li
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
                  <span
                    v-if="(item.count || 1) > 1"
                    class="notification-item__count"
                    :title="`Событий: ${item.count}`"
                  >{{ item.count }}</span>
                  <time class="notification-item__time">{{ relativeTime(item.created_at) }}</time>
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
          </template>

          <!-- Бесшовная подгрузка (#1748 S7): sentinel внизу списка, IntersectionObserver
               триггерит loadMore без кнопки (зеркало Центра/реестров #1158/#1173). -->
          <li
            v-if="hasMoreNotifications"
            :ref="setSentinelRef"
            class="notifications__sentinel"
          >
            <div
              v-if="listLoading"
              class="notifications__spinner notifications__spinner--sm"
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
          </li>
        </ul>
      </div>
    </transition>
  </teleport>

  <NotificationDetailModal
    :show="showDetailModal"
    :notification="detailNotification"
    @close="showDetailModal = false"
    @action="handleDetailAction"
    @unread="handleDetailUnread"
    @delete="handleDetailDelete"
  />
</template>

<script>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { apiRequest } from '@/api/client'
import { getNotificationsPaginated } from '@/api/notifications'
import { useUiStore } from '@/stores/ui'
import eventStream from '@/services/eventStream'
import { useSwipeDismiss } from '@/composables/useSwipeDismiss'
import { useNotificationNavigation } from '@/composables/useNotificationNavigation'
import { useInfiniteList } from '@/composables/useInfiniteList'
import { formatTimeAgo } from '@/utils/datetime'
import { parseNotificationData } from '@/utils/notificationDetails'
import NotificationDetailModal from '@/components/notifications/NotificationDetailModal.vue'
import NotificationFilterTabs from '@/components/notifications/NotificationFilterTabs.vue'
import AppIcon from '@/components/icons/AppIcon.vue'

// Компактная панель (десктоп-дропдаун 360px/350px список, мобильный sheet) - страница
// поменьше, чем у полноэкранных списков (Центр/реестры берут 30, #1158): карточка ниже
// табличной строки, 20 закрывает 3-4 прокрутки панели до новой догрузки.
const NOTIFICATIONS_PER_PAGE = 20

export default {
  name: 'UserNotifications',
  components: { NotificationDetailModal, NotificationFilterTabs, AppIcon },
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
    const { resolveApplicationRoute } = useNotificationNavigation();
    // useInfiniteList (#1158): аккумуляция страниц, hasMore/canLoadMore, устойчивость
    // к ошибкам бэка (#1173, circuit-breaker) - см. buildNotificationsPage/setSentinelRef.
    const infiniteList = useInfiniteList({ perPage: NOTIFICATIONS_PER_PAGE });
    // Глобальный ConfirmDialog нужен и для вопроса об очистке, и для гейтов
    // закрытия панели, пока человек на этот вопрос отвечает.
    const uiStore = useUiStore();
    return {
      uiStore,
      sheetScroll,
      isSheet,
      sheetOffset: swipe.offset,
      sheetDragging: swipe.isDragging,
      onSheetTouchStart: swipe.onTouchStart,
      onSheetTouchMove: swipe.onTouchMove,
      onSheetTouchEnd: swipe.onTouchEnd,
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
    // Escape при открытой модалке подробностей закрывает только её: панель под
    // ней остаётся, иначе одно нажатие схлопывает оба слоя и человек теряет
    // место в списке (#1748).
    //
    // Слушаем в фазе перехвата НАМЕРЕННО: модалка - дочерний компонент, её
    // обработчик навешивается раньше нашего и на том же document, поэтому в
    // обычной фазе он успевает закрыть окно и сбросить showDetailModal до того,
    // как мы его прочитаем. Перехват отдаёт нам событие первыми.
    // Тот же довод про глобальный вопрос подтверждения: он смонтирован в App и
    // ловит Escape в обычной фазе, то есть позже нас. Без гейта одно нажатие
    // закрыло бы панель, оставив вопрос об очистке висеть над пустым местом.
    this.escHandler = (e) => {
      if (e.key === 'Escape' && this.show && !this.showDetailModal && !this.uiStore.confirmState) {
        this.$emit('close')
      }
    }
    document.addEventListener('keydown', this.escHandler, true)
  },
  beforeUnmount() {
    this.disconnectNotificationsSentinel()
    if (this.escHandler) document.removeEventListener('keydown', this.escHandler, true)
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
    // fetchPage для useInfiniteList (#1748 S7): limit/offset + filter текущей вкладки.
    // meta.unread_count независим от страницы/фильтра - обновляет счётчик в шапке
    // на КАЖДОМ ответе (первичном и догрузке), не только на первой странице.
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
        // ошибка первичной загрузки уже отражена в listError (пустой список + сентинел
        // не рисуется без hasMore) - background poll/real-time не должен кидать тост
      }
    },

    setFilter(value) {
      if (this.filter === value) return
      this.filter = value
      this.fetchNotifications()
    },

    // Автодогрузка следующей порции по пересечению sentinel со списком (#1158/#1748 S7).
    // root - сам скроллящийся ul (.notifications__list), не документ: панель компактная,
    // скроллит себя саму и на десктопе, и на мобильном sheet.
    setSentinelRef(el) {
      this.observeNotificationsSentinel(el, this.buildNotificationsPage, { root: this.sheetScroll || null })
    },

    // Вход в настройки прямо из колокольчика: раньше он был только ссылкой в
    // блоке личного кабинета, и найти его оттуда никто не догадывался (#1748).
    openSettings() {
      // Навигация ПЕРЕД закрытием панели: emit('close') снимает v-if родителя и
      // размонтирует этот компонент, а router.push, вызванный из уже снятого
      // компонента, доводит адрес в строке, но не рисует страницу (#1748).
      this.$router.push('/notification-settings')
        .then(() => this.$emit('close'))
        .catch(() => this.$emit('close'))
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
            // meta.unread_count независим от текущей страницы - держим его синхронным
            // локально между рефетчами, иначе счётчик в шапке отстанет до следующего poll.
            this.unreadCount = Math.max(0, this.unreadCount - 1)
          }
        } catch {
          // ignore
        }
      }
      // Клик по карточке раскрывает подробности в модалке (#1748 S6) - переход
      // к заявке теперь делает кнопка действия внутри неё, не сам клик.
      this.detailNotification = item
      this.showDetailModal = true
    },

    // Кнопка действия модалки — то же, что раньше делал клик по карточке о заявке (#973).
    // application_id для навигации лежит в data (jsonb-строка) уведомлений о заявках.
    handleDetailAction() {
      const data = parseNotificationData(this.detailNotification)
      const appId = data.application_id ? Number(data.application_id) : null
      if (!appId) return
      this.showDetailModal = false
      this.$emit('close')
      this.$router.push(this.resolveApplicationRoute(appId)).catch(() => {})
    },

    async handleDetailUnread() {
      const item = this.detailNotification
      if (!item) return
      try {
        const response = await apiRequest(`/notifications/${item.id}/read`, {
          method: 'PUT',
          body: JSON.stringify({ is_read: false }),
        })
        if (response.ok) {
          const wasRead = item.is_read
          item.is_read = false
          if (wasRead) this.unreadCount += 1
        }
      } catch {
        // ignore
      }
      this.showDetailModal = false
    },

    async handleDetailDelete() {
      const item = this.detailNotification
      if (!item) return
      await this.deleteNotification(item.id)
      this.showDetailModal = false
    },

    async deleteNotification(id) {
      try {
        const response = await apiRequest(`/notifications/${id}`, {
          method: 'DELETE',
        })
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

    async clearAll() {
      // Очистка необратима и сносит ВСЕ уведомления пользователя, а не только
      // видимые на текущей вкладке: DeleteAll бьёт по user_id без фильтра.
      const ok = await this.uiStore.confirm({
        title: 'Очистить уведомления?',
        message: 'Все уведомления будут удалены.',
        confirmText: 'Очистить',
        cancelText: 'Отмена',
        danger: false,
      })
      if (!ok) return
      try {
        const response = await apiRequest('/notifications', {
          method: 'DELETE',
        })
        if (response.ok) {
          this.notifications = []
          this.unreadCount = 0
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

    // Обёртка над общим форматтером (@/utils/datetime) - раньше здесь была своя копия
    // логики "N назад" (#1748 S7 dедуп).
    relativeTime(dateStr) {
      return formatTimeAgo(dateStr)
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
  flex-direction: column;
  gap: 8px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--color-border);
  flex-shrink: 0;
}

.notifications__header-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

/* Панель узкая (360px), а в шапке заголовок и три действия. Без nowrap и
   компактного шага подписи переносились по слогам и шапка ехала в две строки. */
.notifications__header-actions {
  display: flex;
  flex-shrink: 0;
  white-space: nowrap;
  align-items: center;
  gap: 10px;
}

.notifications__title {
  margin: 0;
  font-size: 14px;
  white-space: nowrap;
  font-weight: 600;
  color: var(--color-text);
}

.notifications__unread-count {
  font-weight: 400;
  color: var(--accent-text);
}

.notifications__settings-icon {
  /* 19px при общей обводке 1.7 даёт волосок в 1.3px - на мелком значке вес задаёт CSS. */
  stroke-width: 2;
}

.notifications__settings-btn {
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

.notifications__settings-btn:hover {
  color: var(--color-text);
  /* Меньше углового шага зубцов (45): на самом шаге поворот не виден. */
  transform: rotate(22.5deg);
}

.notifications__clear-btn {
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: color 0.2s;
  font-family: 'Montserrat', sans-serif;
  padding: 0;
}

.notifications__clear-btn:hover {
  color: var(--color-text);
}

/* Разделитель дня (#1748 S7) - "Сегодня"/"Вчера"/дата. */
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

/* Счётчик схлопнутых повторов (#1748 S7, count>1) - "3 события" в модалке, здесь
   просто цифра рядом с заголовком: карточка читается как "событий было несколько". */
.notification-item__count {
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
  color: var(--danger-text);
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
  border-top-color: var(--accent-text);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

.notifications__spinner--sm {
  width: 14px;
  height: 14px;
  border-width: 2px;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* Sentinel бесшовной подгрузки (#1748 S7, зеркало #1158/#1173) - невидимая полоса
   внизу списка, пересечение которой триггерит loadMore. */
.notifications__sentinel {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 24px;
  padding: 10px 0;
  flex-shrink: 0;
}

.sentinel-error {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--danger-text);
  font-size: 12px;
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
