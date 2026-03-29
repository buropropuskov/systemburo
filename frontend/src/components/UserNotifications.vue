<template>
    <div class="notifications">
        <div class="notifications__header">
            <div class="notifications__header-left">
                <h3 class="notifications__title">Уведомления</h3>
                <span v-if="unreadCount > 0" class="notification-badge">{{ unreadCount }}</span>
                <div class="filters">
                    <button
                        class="filter-btn"
                        :class="{ active: filter === 'all' }"
                        @click="setFilter('all')"
                    >
                        Все
                    </button>
                    <button
                        class="filter-btn"
                        :class="{ active: filter === 'unread' }"
                        @click="setFilter('unread')"
                    >
                        Непрочитанные
                    </button>
                </div>
            </div>
            <button class="notifications__clear" @click="clearAll">Очистить</button>
        </div>
        
        <div class="notifications__list" :class="{ 'empty-list': filteredNotifications.length === 0 && !loading }">
            <div v-if="loading && filteredNotifications.length === 0" class="notifications__loading">
                <div class="loader"></div>
                <span>Загрузка...</span>
            </div>
            <div v-else-if="filteredNotifications.length === 0" class="notifications__empty">
                
                <p>Уведомлений нет</p>
            </div>
            <div v-else class="notifications__items">
                <div
                    v-for="notif in filteredNotifications"
                    :key="notif.id"
                    class="notification-item"
                    :class="{ unread: !notif.is_read }"
                >
                    <div class="notification-dot-wrapper">
                        <transition name="dot-fade">
                            <div v-if="!notif.is_read" class="notification-dot"></div>
                        </transition>
                    </div>
                    <div class="notification-content" @click="markRead(notif)">
                        <div class="notification-header">
                            <div class="notification-title">{{ notif.title }}</div>
                            <div class="notification-date">{{ formatDate(notif.created_at) }}</div>
                        </div>
                        <div class="notification-message">{{ notif.message }}</div>
                    </div>
                    <button class="delete-btn" @click.stop="deleteNotification(notif.id)">
                        <svg width="10" height="10" viewBox="0 0 14 14" fill="none">
                            <path d="M13 1L1 13M1 1L13 13" stroke="#a2a2a2" stroke-width="2" stroke-linecap="round"/>
                        </svg>
                    </button>
                </div>
            </div>
        </div>
    </div>
</template>

<script>
export default {
    props: {
        userId: {
            type: Number,
            required: true
        }
    },
    data() {
        return {
            notifications: [],
            loading: false,
            interval: null,
            filter: 'all'
        };
    },
    computed: {
        unreadCount() {
            return this.notifications.filter(n => !n.is_read).length;
        },
        filteredNotifications() {
            if (this.filter === 'unread') {
                return this.notifications.filter(n => !n.is_read);
            }
            return this.notifications;
        }
    },
    watch: {
        userId: {
            immediate: true,
            handler(newVal) {
                if (newVal) {
                    this.fetchNotifications();
                }
            }
        }
    },
    mounted() {
        this.interval = setInterval(this.fetchNotifications, 30000);
        if (this.$bus) {
            this.$bus.on('notification-updated', this.fetchNotifications);
        }
    },
    beforeUnmount() {
        if (this.interval) clearInterval(this.interval);
        if (this.$bus) {
            this.$bus.off('notification-updated', this.fetchNotifications);
        }
    },
    methods: {
        async fetchNotifications() {
            if (!this.userId) return;
            this.loading = true;
            try {
                const token = localStorage.getItem('token');
                const res = await fetch('http://localhost:8080/notifications', {
                    headers: { Authorization: `Bearer ${token}` }
                });
                if (res.ok) {
                    this.notifications = await res.json();
                }
            } catch (err) {
                console.error('Ошибка загрузки уведомлений', err);
            } finally {
                this.loading = false;
            }
        },
        async markRead(notif) {
            if (notif.is_read) return;
            try {
                const token = localStorage.getItem('token');
                const res = await fetch(`http://localhost:8080/notifications/${notif.id}/read`, {
                    method: 'PUT',
                    headers: {
                        Authorization: `Bearer ${token}`,
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({ is_read: true })
                });
                if (res.ok) {
                    notif.is_read = true;
                    if (this.$bus) this.$bus.emit('notification-updated');
                }
            } catch (err) {
                console.error('Ошибка отметки прочитанным', err);
            }
        },
        async deleteNotification(id) {
            try {
                const token = localStorage.getItem('token');
                const res = await fetch(`http://localhost:8080/notifications/${id}`, {
                    method: 'DELETE',
                    headers: { Authorization: `Bearer ${token}` }
                });
                if (res.ok) {
                    this.notifications = this.notifications.filter(n => n.id !== id);
                    if (this.$bus) this.$bus.emit('notification-updated');
                }
            } catch (err) {
                console.error('Ошибка удаления уведомления', err);
            }
        },
        async clearAll() {
            if (!confirm('Вы уверены, что хотите удалить все уведомления?')) return;
            try {
                const token = localStorage.getItem('token');
                const res = await fetch('http://localhost:8080/notifications', {
                    method: 'DELETE',
                    headers: { Authorization: `Bearer ${token}` }
                });
                if (res.ok) {
                    this.notifications = [];
                    if (this.$bus) this.$bus.emit('notification-updated');
                }
            } catch (err) {
                console.error('Ошибка очистки уведомлений', err);
            }
        },
        setFilter(value) {
            this.filter = value;
        },
        formatDate(dateString) {
            const date = new Date(dateString);
            const now = new Date();
            const diffMs = now - date;
            const diffSec = Math.floor(diffMs / 1000);
            const diffMin = Math.floor(diffSec / 60);
            const diffHour = Math.floor(diffMin / 60);
            const diffDay = Math.floor(diffHour / 24);

            if (diffSec < 60) return 'только что';
            if (diffMin < 60) return `${diffMin} мин. назад`;
            if (diffHour < 24) return `${diffHour} ч. назад`;
            if (diffDay === 1) return 'вчера';
            return date.toLocaleDateString('ru-RU');
        }
    }
}
</script>

<style scoped>
.notifications {
    height: 200px;
    width: 38%;
    background-color: #FFF;
    border-radius: 30px;
    border: 1px solid #e6e6e6;
    box-shadow: 0 3px 10px rgba(0, 0, 0, 0.05);
    display: flex;
    flex-direction: column;
    overflow: hidden;
}

.notifications__header {
    height: 48px;
    width: 100%;
    border-bottom: 1px solid #f0f0f0;
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0 20px;
    flex-shrink: 0;
    background: #fff;
}

.notifications__header-left {
    display: flex;
    align-items: center;
    gap: 12px;
}

.notifications__title {
    font-size: 14px;
    font-weight: 600;
    margin: 0;
    color: #1a1a1a;
}

.notification-badge {
    background: #f44336;
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
    margin-left: 8px;
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
}

.filter-btn.active {
    color: #4F5BDF;
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
    transition: all 0.2s ease;
    font-weight: 500;
    padding: 4px 8px;
    border-radius: 20px;
}

.notifications__clear:hover {
    color: #666;
    background: #f5f5f5;
}

.notifications__list {
    flex: 1;
    overflow-y: auto;
    scrollbar-width: thin;
}

.notifications__list::-webkit-scrollbar {
    width: 4px;
}

.notifications__list::-webkit-scrollbar-track {
    background: #f0f0f0;
    border-radius: 2px;
}

.notifications__list::-webkit-scrollbar-thumb {
    background: #e6e6e6;
    border-radius: 2px;
}

.empty-list {
    display: flex;
    align-items: center;
    justify-content: center;
}

.notifications__loading {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    padding: 40px 20px;
    color: #a2a2a2;
    font-size: 13px;
}

.loader {
    width: 24px;
    height: 24px;
    border: 2px solid #f0f0f0;
    border-top: 2px solid #4F5BDF;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
}

@keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
}

.notifications__empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    padding: 40px 20px;
    text-align: center;
}

.notifications__empty svg {
    opacity: 0.5;
}

.notifications__empty p {
    margin: 0;
    font-size: 13px;
    color: #a2a2a2;
}

.notifications__items {
    display: flex;
    flex-direction: column;
}

.notification-item {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 12px 20px;
    border-bottom: 1px solid #f0f0f0;
    transition: background 0.2s ease;
    cursor: pointer;
    transition: all 0.2s ease;
}

.notification-item:hover {
    background: #fafafa;
}

.notification-item.unread {
    background: #f8f9ff;
}

.notification-dot-wrapper {
    width: 8px;
    flex-shrink: 0;
    margin-top: 6px;
    transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.notification-item:not(.unread) .notification-dot-wrapper {
    width: 0;
}

.notification-dot {
    width: 8px;
    height: 8px;
    background: #4F5BDF;
    border-radius: 50%;
}

/* Анимация для точки */
.dot-fade-enter-active,
.dot-fade-leave-active {
    transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.dot-fade-enter-from,
.dot-fade-leave-to {
    opacity: 0;
    transform: scale(0);
}

.dot-fade-enter-to,
.dot-fade-leave-from {
    opacity: 1;
    transform: scale(1);
}

.notification-content {
    flex: 1;
    overflow: hidden;
    transition: margin-left 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.notification-item.unread .notification-content {
    margin-left: 0;
}

.notification-item:not(.unread) .notification-content {
    margin-left: -10px;
}

.notification-header {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    margin-bottom: 6px;
    gap: 12px;
}

.notification-title {
    font-size: 13px;
    font-weight: 500;
    color: #333;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    flex: 1;
    transition: all 0.25s ease;
}

.notification-item.unread .notification-title {
    font-weight: 600;
    color: #4F5BDF;
}

.notification-date {
    font-size: 10px;
    color: #a2a2a2;
    white-space: nowrap;
    flex-shrink: 0;
}

.notification-message {
    font-size: 12px;
    color: #666;
    line-height: 1.4;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
}

.delete-btn {
    background: none;
    border: none;
    cursor: pointer;
    padding: 4px;
    border-radius: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.2s ease;
    flex-shrink: 0;
    opacity: 0;
    pointer-events: none;
}

.notification-item:hover .delete-btn {
    opacity: 1;
    pointer-events: auto;
}

.delete-btn:hover {
    background: #f5f5f5;
}

.delete-btn:hover svg path {
    stroke: #666;
}

@media (max-width: 768px) {
    .notifications {
        width: 100%;
        height: auto;
        min-height: 200px;
    }
    
    .notifications__header {
        padding: 0 16px;
        height: 44px;
    }
    
    .notification-item {
        padding: 10px 16px;
        gap: 10px;
    }
    
    .notification-dot-wrapper {
        width: 6px;
        margin-top: 5px;
    }
    
    .notification-item:not(.unread) .notification-dot-wrapper {
        width: 0;
    }
    
    .notification-item:not(.unread) .notification-content {
        margin-left: -6px;
    }
    
    .notification-dot {
        width: 6px;
        height: 6px;
    }
    
    .notification-header {
        flex-wrap: wrap;
        gap: 6px;
    }
    
    .notification-title {
        white-space: normal;
        line-height: 1.3;
    }
    
    .delete-btn {
        opacity: 1;
        pointer-events: auto;
    }
}
</style>