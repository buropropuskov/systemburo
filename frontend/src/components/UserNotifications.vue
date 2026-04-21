<template>
  <transition name="panel-slide">
    <div v-if="show" class="notifications" @click.stop>
      <header class="notifications__header">
        <h3 class="notifications__title">
          Уведомления
          <span v-if="unreadCount > 0" class="notifications__unread-count">({{ unreadCount }})</span>
        </h3>
        <button
          v-if="notifications.length > 0"
          class="notifications__clear-btn"
          @click="clearAll"
        >
          Очистить
        </button>
      </header>

      <div v-if="loading && notifications.length === 0" class="notifications__loading">
        <div class="notifications__spinner"></div>
      </div>

      <div v-else-if="notifications.length === 0" class="notifications__empty">
        <p class="notifications__empty-text">Нет уведомлений</p>
      </div>

      <ul v-else class="notifications__list" role="list">
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
              <p v-if="item.title" class="notification-item__title">{{ item.title }}</p>
              <time class="notification-item__time">{{ timeAgo(item.created_at) }}</time>
            </div>
            <p v-if="item.message" class="notification-item__message">{{ item.message }}</p>
          </div>
          <button
            class="notification-item__delete"
            @click.stop="deleteNotification(item.id)"
            aria-label="Удалить уведомление"
          >
            &times;
          </button>
        </li>
      </ul>
    </div>
  </transition>
</template>

<script>
import { apiRequest } from '@/api/client'

export default {
  name: 'UserNotifications',
  props: {
    show: {
      type: Boolean,
      default: false,
    },
  },
  emits: ['update:unread-count'],
  data() {
    return {
      notifications: [],
      loading: false,
      pollTimer: null,
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
        this.startPolling()
      } else {
        this.stopPolling()
      }
    },
    unreadCount(count) {
      this.$emit('update:unread-count', count)
    },
  },
  beforeUnmount() {
    this.stopPolling()
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
      if (item.is_read) return
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
  top: 50px;
  right: 80px;
  width: 360px;
  background: #fff;
  border-radius: 20px;
  border: 1px solid var(--color-border);
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.12);
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
  color: var(--color-primary);
}

.notifications__clear-btn {
  background: none;
  border: none;
  color: #a2a2a2;
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
  background: #d9e2ff;
  border-radius: 2px;
}

/* Notification item */
.notification-item {
  display: flex;
  align-items: flex-start;
  padding: 10px 16px;
  cursor: pointer;
  transition: background-color 0.15s;
  border-bottom: 1px solid #f5f5f5;
}

.notification-item:last-child {
  border-bottom: none;
}

.notification-item:hover {
  background: #fafafa;
}

.notification-item--unread {
  background: #f0f1ff;
}

.notification-item--unread:hover {
  background: #e8e9ff;
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
  color: #a2a2a2;
  white-space: nowrap;
  flex-shrink: 0;
}

.notification-item__message {
  margin: 4px 0 0;
  font-size: 12px;
  color: #666;
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
  color: #ccc;
  font-size: 18px;
  cursor: pointer;
  border-radius: 50%;
  transition: all 0.15s;
  margin-left: 8px;
  margin-top: 2px;
}

.notification-item__delete:hover {
  color: var(--color-danger);
  background: #fde8e8;
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
  border-top-color: var(--color-primary);
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
  color: #a2a2a2;
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

/* Responsive */
@media (max-width: 576px) {
  .notifications {
    width: calc(100vw - 20px);
    right: 10px;
    top: 55px;
  }
}
</style>
