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
        <div class="filters">
          <button
            type="button"
            class="filter-btn"
            :class="{ active: filter === 'all' }"
            data-testid="cabinet-notifications-filter-all"
            @click="setFilter('all')"
          >
            Все
          </button>
          <button
            type="button"
            class="filter-btn"
            :class="{ active: filter === 'unread' }"
            data-testid="cabinet-notifications-filter-unread"
            @click="setFilter('unread')"
          >
            Непрочитанные
          </button>
        </div>
      </div>
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

    <div
      class="notifications__list"
      :class="{ 'empty-list': filteredNotifications.length === 0 && !loading }"
    >
      <div
        v-if="loading && filteredNotifications.length === 0"
        class="notifications__loading"
      >
        <LoaderSpinner />
      </div>
      <div
        v-else-if="filteredNotifications.length === 0"
        class="notifications__empty"
      >
        <p>Уведомлений нет</p>
      </div>
      <div
        v-else
        class="notifications__items"
      >
        <div
          v-for="notif in filteredNotifications"
          :key="notif.id"
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
              <div class="notification-date">
                {{ formatDate(notif.created_at) }}
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
      </div>
    </div>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client'
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue'

export default {
  name: 'UserNotificationsInline',

  components: { LoaderSpinner },

  data() {
    return {
      notifications: [],
      loading: false,
      pollTimer: null,
      filter: 'all',
    }
  },

  computed: {
    unreadCount() {
      return this.notifications.filter(n => !n.is_read).length
    },
    filteredNotifications() {
      if (this.filter === 'unread') {
        return this.notifications.filter(n => !n.is_read)
      }
      return this.notifications
    },
  },

  mounted() {
    this.fetchNotifications()
    this.pollTimer = setInterval(this.fetchNotifications, 30000)
  },

  beforeUnmount() {
    if (this.pollTimer) {
      clearInterval(this.pollTimer)
      this.pollTimer = null
    }
  },

  methods: {
    async fetchNotifications() {
      this.loading = true
      try {
        const response = await apiRequest('/notifications')
        if (response.ok) {
          this.notifications = (await response.json()) || []
        }
      } catch {
        // background poll - не показываем ошибку
      } finally {
        this.loading = false
      }
    },

    async markRead(notif) {
      if (notif.is_read) return
      try {
        const response = await apiRequest(`/notifications/${notif.id}/read`, {
          method: 'PUT',
          body: JSON.stringify({ is_read: true }),
        })
        if (response.ok) {
          notif.is_read = true
        }
      } catch {
        // ignore
      }
    },

    async deleteNotification(id) {
      try {
        const response = await apiRequest(`/notifications/${id}`, { method: 'DELETE' })
        if (response.ok) {
          this.notifications = this.notifications.filter(n => n.id !== id)
        }
      } catch {
        // ignore
      }
    },

    async clearAll() {
      if (!window.confirm('Вы уверены, что хотите удалить все уведомления?')) return
      try {
        const response = await apiRequest('/notifications', { method: 'DELETE' })
        if (response.ok) {
          this.notifications = []
        }
      } catch {
        // ignore
      }
    },

    setFilter(value) {
      this.filter = value
    },

    formatDate(dateString) {
      if (!dateString) return ''
      const date = new Date(dateString)
      const now = new Date()
      const diffSec = Math.floor((now - date) / 1000)
      const diffMin = Math.floor(diffSec / 60)
      const diffHour = Math.floor(diffMin / 60)
      const diffDay = Math.floor(diffHour / 24)

      if (diffSec < 60) return 'только что'
      if (diffMin < 60) return `${diffMin} мин. назад`
      if (diffHour < 24) return `${diffHour} ч. назад`
      if (diffDay === 1) return 'вчера'
      return date.toLocaleDateString('ru-RU')
    },
  },
}
</script>

<style scoped>
.notifications {
  flex: 1;
  min-height: 200px;
  max-height: 200px;
  background-color: #fff;
  border-radius: 30px;
  border: 1px solid #e6e6e6;
  box-shadow: 0 3px 10px rgba(0, 0, 0, 0.05);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.notifications__header {
  min-height: 48px;
  width: 100%;
  border-bottom: 1px solid #f0f0f0;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 20px;
  flex-shrink: 0;
  background: #fff;
  gap: 12px;
}

.notifications__header-left {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.notifications__title {
  font-size: 14px;
  font-weight: 600;
  margin: 0;
  color: #1a1a1a;
}

.notification-badge {
  background: var(--color-danger, #f44336);
  color: white;
  border-radius: 30px;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 600;
  min-width: 20px;
  text-align: center;
}

.filters {
  display: flex;
  gap: 4px;
}

.filter-btn {
  background: none;
  border: none;
  font-size: 12px;
  color: #a2a2a2;
  cursor: pointer;
  padding: 4px 12px;
  border-radius: 30px;
  transition: all 0.2s ease;
  font-weight: 500;
  font-family: inherit;
}

.filter-btn.active {
  color: var(--color-primary);
  background: rgba(79, 91, 223, 0.08);
}

.filter-btn:hover:not(.active) {
  color: #666;
  background: #f5f5f5;
}

.notifications__clear {
  background: none;
  border: none;
  color: #a2a2a2;
  font-size: 12px;
  cursor: pointer;
  transition: color 0.2s ease;
  font-weight: 500;
  padding: 4px 8px;
  border-radius: 20px;
  font-family: inherit;
}

.notifications__clear:hover {
  color: var(--color-text);
  background: #f5f5f5;
}

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
  background: #d9e2ff;
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
  border-bottom: 1px solid #f5f5f5;
  transition: background-color 0.15s;
}

.notification-item:last-child {
  border-bottom: none;
}

.notification-item.unread {
  background: rgba(79, 91, 223, 0.03);
}

.notification-dot-wrapper {
  width: 10px;
  display: flex;
  justify-content: center;
  padding-top: 6px;
  flex-shrink: 0;
}

.notification-item:not(.unread) .notification-dot-wrapper {
  display: none;
}

.notification-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--color-primary);
}

.dot-fade-enter-active,
.dot-fade-leave-active {
  transition: opacity 0.2s;
}
.dot-fade-enter-from,
.dot-fade-leave-to {
  opacity: 0;
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

.notification-date {
  font-size: 11px;
  color: #a2a2a2;
  white-space: nowrap;
  flex-shrink: 0;
}

.notification-message {
  margin: 4px 0 0;
  font-size: 12px;
  color: #666;
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
  background: #fde8e8;
}

.delete-btn:hover svg path {
  stroke: var(--color-danger, #dc2626);
}

.notifications__loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
}

.notifications__empty {
  padding: 40px;
  color: #a2a2a2;
  font-size: 13px;
  text-align: center;
}

.notifications__empty p {
  margin: 0;
}
</style>
