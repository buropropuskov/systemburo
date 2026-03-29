<template>
  <header class="header" ref="header">
    <div class="header__title">
      <h3>Добрый день, {{ displayName }}!</h3>
      <p class="header__subtitle">Мы рады, что вы здесь!</p>
    </div>

    <div class="header__info">
      <button class="feedback-btn" @click="openFeedbackModal">Сообщить о проблеме</button>
      <button
        class="broadcast"
        @click="openAnnouncementModal"
        :class="{ 'important-announcement': activeAnnouncement?.is_important }"
        v-if="activeAnnouncement"
      >
        {{ activeAnnouncement.is_important ? 'Важное объявление' : 'Объявление' }}
      </button>
      <p class="time">{{ currentDateTime }}</p>
      <div class="language-selector"><p class="current-language">RU</p></div>
      <div class="user__notifications" @click.stop="toggleNotifications">
        <img src="@/assets/icons/notifications.png" class="notifications__icon" alt="Уведомления" />
        <span v-if="unreadCount > 0" class="notification-badge">{{ unreadCount }}</span>
      </div>
      <div class="appl-btn__container">
        <button class="appl-btn" @click="navigateToSubmit" :class="{ 'appl-btn--fixed': isHeaderHidden }">Подать заявку</button>
      </div>
    </div>

    <transition name="fade-slide">
      <div v-if="showNotificationsPanel" class="notifications-panel" @click.stop>
        <div class="notifications-panel-header"><h3>Уведомления</h3><button class="clear-all-btn" @click="clearAllNotifications">Очистить</button></div>
        <div class="notifications-panel-list">
          <div v-if="loadingNotifications && panelNotifications.length === 0" class="loading-placeholder"><div class="loader"></div><span>Загрузка...</span></div>
          <div v-else-if="panelNotifications.length === 0" class="empty-placeholder">
            <p>Уведомлений нет</p>
          </div>
          <div v-else class="notifications-items">
            <div v-for="notif in panelNotifications" :key="notif.id" class="panel-notification-item" :class="{ unread: !notif.is_read }">
              <div class="notification-dot-wrapper"><transition name="dot-fade"><div v-if="!notif.is_read" class="notification-dot"></div></transition></div>
              <div class="panel-notification-content" @click="markReadPanel(notif)">
                <div class="panel-notification-header"><div class="panel-notification-title">{{ notif.title }}</div><div class="panel-notification-date">{{ formatDate(notif.created_at) }}</div></div>
                <div class="panel-notification-message">{{ notif.message }}</div>
              </div>
              <button class="panel-delete-btn" @click.stop="deletePanelNotification(notif.id)"><svg width="10" height="10" viewBox="0 0 14 14" fill="none"><path d="M13 1L1 13M1 1L13 13" stroke="#a2a2a2" stroke-width="2" stroke-linecap="round"/></svg></button>
            </div>
          </div>
        </div>
      </div>
    </transition>

    <FeedbackModal v-model:show="showFeedbackModal" @submitted="handleFeedbackSubmitted" />
    <AnnouncementModal v-model:show="showAnnouncementModal" :announcement="activeAnnouncement" @close="closeAnnouncementModal" />
  </header>
</template>

<script>
import FeedbackModal from '@/components/FeedbackModal.vue'
import AnnouncementModal from '@/components/AnnouncementModal.vue'

export default {
  name: 'TheHeader',
  components: { FeedbackModal, AnnouncementModal },
  data() {
    return {
      userFirstName: '',
      userLastName: '',
      currentDateTime: '',
      timer: null,
      isHeaderHidden: false,
      observer: null,
      showFeedbackModal: false,
      showNotificationsPanel: false,
      userId: null,
      panelNotifications: [],
      unreadCount: 0,
      loadingNotifications: false,
      panelInterval: null,
      activeAnnouncement: null,
      showAnnouncementModal: false,
      announcementRefreshInterval: null
    }
  },
  computed: {
    displayName() { return this.userFirstName || this.userLastName || '' }
  },
  watch: { '$route'() { this.fetchUserData() } },
  methods: {
    openFeedbackModal() { this.showFeedbackModal = true },
    handleFeedbackSubmitted() {
  
        if (this.$route.path === '/feedback') this.$emit('refresh-feedback') },
    openAnnouncementModal() { if (this.activeAnnouncement) this.showAnnouncementModal = true },
    closeAnnouncementModal() { this.showAnnouncementModal = false },
    navigateToSubmit() { this.$router.push('/submit-form') },
    async fetchUserData() {
      try {
        const token = localStorage.getItem('token')
        if (!token) { console.log('Пользователь не авторизован'); return }
        const response = await fetch('http://localhost:8080/users/me', { headers: { Authorization: `Bearer ${token}` } })
        if (response.ok) {
          const userData = await response.json()
          this.userFirstName = userData.first_name || ''
          this.userLastName = userData.last_name || ''
          this.userId = userData.id
          this.fetchPanelNotifications()
          this.fetchActiveAnnouncement()
        } else console.error('Ошибка при загрузке данных пользователя')
      } catch (error) { console.error('Ошибка сети при загрузке данных пользователя:', error) }
    },
    async fetchActiveAnnouncement() {
      try {
        const token = localStorage.getItem('token')
        const response = await fetch('http://localhost:8080/announcements/active', { headers: { Authorization: `Bearer ${token}` } })
        if (response.ok) {
          const data = await response.json()
          this.activeAnnouncement = data
          if (this.$bus) this.$bus.emit('announcement-updated', data)
         
        } else {
          this.activeAnnouncement = null
          if (this.$bus) this.$bus.emit('announcement-updated', null)
         
        }
      } catch (error) { console.error('Ошибка загрузки активного объявления:', error) }
    },
    updateDateTime() {
      const now = new Date()
      this.currentDateTime = `${String(now.getDate()).padStart(2,'0')}.${String(now.getMonth()+1).padStart(2,'0')}.${now.getFullYear()} ${String(now.getHours()).padStart(2,'0')}:${String(now.getMinutes()).padStart(2,'0')}:${String(now.getSeconds()).padStart(2,'0')}`
    },
    startDateTimeTimer() { this.updateDateTime(); this.timer = setInterval(this.updateDateTime, 1000) },
    initIntersectionObserver() {
      this.observer = new IntersectionObserver((entries) => { entries.forEach(entry => { this.isHeaderHidden = !entry.isIntersecting }) }, { threshold: 0 })
      if (this.$refs.header) this.observer.observe(this.$refs.header)
    },
    toggleNotifications() {
      this.showNotificationsPanel = !this.showNotificationsPanel
      if (this.showNotificationsPanel) { this.fetchPanelNotifications(); if (this.panelInterval) clearInterval(this.panelInterval); this.panelInterval = setInterval(this.fetchPanelNotifications, 30000) }
      else if (this.panelInterval) clearInterval(this.panelInterval)
    },
    async fetchPanelNotifications() {
      if (!this.userId) return
      this.loadingNotifications = true
      try {
        const token = localStorage.getItem('token')
        const res = await fetch('http://localhost:8080/notifications', { headers: { Authorization: `Bearer ${token}` } })
        if (res.ok) { const data = await res.json(); this.panelNotifications = data; this.unreadCount = data.filter(n => !n.is_read).length }
      } catch (err) { console.error('Ошибка загрузки уведомлений', err) } finally { this.loadingNotifications = false }
    },
    async markReadPanel(notif) {
      if (notif.is_read) return
      try {
        const token = localStorage.getItem('token')
        const res = await fetch(`http://localhost:8080/notifications/${notif.id}/read`, { method: 'PUT', headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }, body: JSON.stringify({ is_read: true }) })
        if (res.ok) { notif.is_read = true; this.unreadCount = this.panelNotifications.filter(n => !n.is_read).length; if (this.$bus) this.$bus.emit('notification-updated') }
      } catch (err) { console.error('Ошибка отметки прочитанным', err) }
    },
    async deletePanelNotification(id) {
      try {
        const token = localStorage.getItem('token')
        const res = await fetch(`http://localhost:8080/notifications/${id}`, { method: 'DELETE', headers: { Authorization: `Bearer ${token}` } })
        if (res.ok) { this.panelNotifications = this.panelNotifications.filter(n => n.id !== id); this.unreadCount = this.panelNotifications.filter(n => !n.is_read).length; if (this.$bus) this.$bus.emit('notification-updated') }
      } catch (err) { console.error('Ошибка удаления уведомления', err) }
    },
    async clearAllNotifications() {
      if (!confirm('Вы уверены, что хотите удалить все уведомления?')) return
      try {
        const token = localStorage.getItem('token')
        const res = await fetch('http://localhost:8080/notifications', { method: 'DELETE', headers: { Authorization: `Bearer ${token}` } })
        if (res.ok) { this.panelNotifications = []; this.unreadCount = 0; if (this.$bus) this.$bus.emit('notification-updated') }
      } catch (err) { console.error('Ошибка очистки уведомлений', err) }
    },
    formatDate(dateString) {
      const date = new Date(dateString)
      const diffSec = Math.floor((new Date() - date) / 1000)
      if (diffSec < 60) return 'только что'
      if (diffSec < 3600) return `${Math.floor(diffSec / 60)} мин назад`
      if (diffSec < 86400) return `${Math.floor(diffSec / 3600)} ч назад`
      if (diffSec < 172800) return 'вчера'
      return date.toLocaleDateString('ru-RU')
    }
  },
  mounted() {
    this.fetchUserData()
    this.startDateTimeTimer()
    this.$nextTick(() => this.initIntersectionObserver())
    if (this.$bus) {
      this.$bus.on('notification-updated', this.fetchPanelNotifications)
      this.$bus.on('announcement-updated', (announcement) => { this.activeAnnouncement = announcement })
    }
    this.announcementRefreshInterval = setInterval(() => this.fetchActiveAnnouncement(), 5000)
    setInterval(() => { if (!this.showNotificationsPanel && this.userId) this.fetchPanelNotifications() }, 60000)
    document.addEventListener('click', (e) => { if (this.showNotificationsPanel && !this.$el.contains(e.target)) { this.showNotificationsPanel = false; if (this.panelInterval) clearInterval(this.panelInterval) } })
  },
  beforeUnmount() {
    if (this.timer) clearInterval(this.timer)
    if (this.observer) this.observer.disconnect()
    if (this.panelInterval) clearInterval(this.panelInterval)
    if (this.announcementRefreshInterval) clearInterval(this.announcementRefreshInterval)
    if (this.$bus) { this.$bus.off('notification-updated', this.fetchPanelNotifications); this.$bus.off('announcement-updated') }
    document.removeEventListener('click', this.closeNotificationsPanel)
  }
}
</script>



<style scoped>
/* Все стили остаются без изменений */
h3 {
  font-size: 16px;
}

.header {
  width: 100%;
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid #e6e6e6;
  padding: 0 20px;
  position: relative;
  z-index: 100;
}

.header__title {
  display:flex;
  flex-direction: column;
  gap: 0px;
}

.header__subtitle {
  font-size: 12px;
  color: #a2a2a2;
  font-weight: 500;
}

.header__info {
  display: flex;
  align-items: center;
  gap: 15px;
  position: relative;
}

.feedback-btn {
  height: 35px;
  font-size: 14px;
  color: #6E4A3A;
  border: none;
  outline: none;
  font-weight: 500;
  background: transparent;
  cursor: pointer;
  white-space: nowrap;
  padding: 0 15px;
  text-decoration: underline;
  text-decoration-color: transparent;
  transition: text-decoration-color 0.2s ease;
  text-underline-position: under;
}

.feedback-btn:hover {
  text-decoration-color: #6E4A3A;
}

.broadcast {
  width: fit-content;
  padding: 0 15px;
  height: 35px;
  font-size: 14px;
  color: #6E4A3A;
  border: 1px solid #e6e6e6;
  outline: none;
  border-radius: 50px;
  font-weight: 500;
  background: linear-gradient(to right, rgba(255,255,240,1), rgba(255,246,217,1));
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.2s ease;
}

.broadcast.important-announcement {
  background: linear-gradient(to right, rgba(255,235,235,1), rgba(255,215,215,1));
  color: #c62828;
  border-color: #ffcdd2;
}

.broadcast:hover {
  background: linear-gradient(to right, rgba(255,250,220,1), rgba(255,240,200,1));
}

.broadcast.important-announcement:hover {
  background: linear-gradient(to right, rgba(255,220,220,1), rgba(255,200,200,1));
}

.time {
  font-size: 16px;
  color: #a2a2a2;
  min-width: 160px;
  text-align: center;
  font-variant-numeric: tabular-nums;
}

.current-language {
  font-size: 16px;
  font-weight: 500;
  color: #a2a2a2;
  cursor: pointer;
}

.current-language:hover {
  color: #333
}

.user__notifications {
  position: relative;
  width: fit-content;
  height: 35px;
  border-radius: 50px;
  padding: 0 15px;
  display: flex;
  gap:20px;
  align-items: center;
  justify-content: center;
  border: 1px solid #e6e6e6;
  box-shadow: 0 2px 2px rgba(0,0,0,0.05);
  cursor: pointer;
}

.notifications__icon {
  width: 20px;
  height: 20px;
  cursor: pointer;
}

.notifications__icon:hover {
  filter: contrast(0.01);
}

.notification-badge {
  position: absolute;
  top: -5px;
  right: -5px;
  background-color: #f44336;
  color: white;
  border-radius: 50%;
  padding: 2px 6px;
  font-size: 12px;
  font-weight: bold;
  min-width: 18px;
  text-align: center;
}

.appl-btn__container {
  width: 155px;
  height: 30px;
  border-radius: 50px;
  background-color: #f2f2f2;
}

.appl-btn {
  position: fixed;
  height: 30px;
  width: fit-content;
  padding: 0 20px;
  font-size: 15px;
  color: #000;
  background-color: #fff;
  border: 1px solid #4F5BDF;
  outline: none;
  cursor: pointer;
  font-weight: 400;
  border-radius: 15px;
  transition: .2s;
  box-shadow: 0 2px 2px rgba(0,0,0,0.05);
}

.appl-btn:hover {
  background-color: #e6e6e6;
}

.appl-btn--fixed {
  position: fixed;
  z-index: 1000;
}

.appl-btn--fixed {
  position: fixed;
  top: 20px;
  right: 20px;
  z-index: 999;
  animation: slide-down 0.3s ease;
}

@keyframes slide-down {
  from {
    transform: translateY(-20px);
    opacity: 0;
  }
  to {
    transform: translateY(0);
    opacity: 1;
  }
}

/* Анимация для окошка уведомлений */
.fade-slide-enter-active,
.fade-slide-leave-active {
  transition: all 0.3s ease;
}
.fade-slide-enter-from,
.fade-slide-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}
.fade-slide-enter-to,
.fade-slide-leave-from {
  opacity: 1;
  transform: translateY(0);
}

/* Панель уведомлений */
.notifications-panel {
  position: absolute;
  top: 75px;
  right: 15px;
  width: 470px;
  max-height: 215px;
  background: white;
  border-radius: 30px;
  box-shadow: 0 3px 12px rgba(0,0,0,0.15);
  z-index: 2000;
  border: 1px solid #e6e6e6;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.notifications-panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 5px 20px;
  border-bottom: 1px solid #f0f0f0;
  background: #fff;
}

.notifications-panel-header h3 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: #1a1a1a;
}

.clear-all-btn {
  background: none;
  border: none;
  color: #a2a2a2;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
  font-weight: 500;
  padding: 4px 8px;
  border-radius: 20px;
}

.clear-all-btn:hover {
  color: #666;
  background: #f5f5f5;
}

.notifications-panel-list {
  flex: 1;
  overflow-y: auto;
  max-height: 420px;
  scrollbar-width: thin;
}

.notifications-panel-list::-webkit-scrollbar {
  width: 4px;
}

.notifications-panel-list::-webkit-scrollbar-track {
  background: #f0f0f0;
  border-radius: 2px;
}

.notifications-panel-list::-webkit-scrollbar-thumb {
  background: #e6e6e6;
  border-radius: 2px;
}

.loading-placeholder, .empty-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 60px 20px;
  text-align: center;
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

.empty-placeholder svg {
  opacity: 0.5;
}

.empty-placeholder p {
  margin: 0;
}

.notifications-items {
  display: flex;
  flex-direction: column;
}

.panel-notification-item {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px 20px;
  border-bottom: 1px solid #f0f0f0;
  transition: background 0.2s ease;
  cursor: pointer;
}

.panel-notification-item:hover {
  background: #fafafa;
}

.panel-notification-item.unread {
  background: #f8f9ff;
}

.notification-dot-wrapper {
  width: 8px;
  flex-shrink: 0;
  margin-top: 6px;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.panel-notification-item:not(.unread) .notification-dot-wrapper {
  width: 0;
}

.notification-dot {
  width: 8px;
  height: 8px;
  background: #4F5BDF;
  border-radius: 50%;
}

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

.panel-notification-content {
  flex: 1;
  overflow: hidden;
  transition: margin-left 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.panel-notification-item.unread .panel-notification-content {
  margin-left: 0;
}

.panel-notification-item:not(.unread) .panel-notification-content {
  margin-left: -8px;
}

.panel-notification-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 6px;
  gap: 12px;
}

.panel-notification-title {
  font-size: 13px;
  font-weight: 500;
  color: #333;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

.panel-notification-item.unread .panel-notification-title {
  font-weight: 600;
  color: #4F5BDF;
}

.panel-notification-date {
  font-size: 10px;
  color: #a2a2a2;
  white-space: nowrap;
  flex-shrink: 0;
}

.panel-notification-message {
  font-size: 12px;
  color: #666;
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.panel-delete-btn {
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

.panel-notification-item:hover .panel-delete-btn {
  opacity: 1;
  pointer-events: auto;
}

.panel-delete-btn:hover {
  background: #f5f5f5;
}

.panel-delete-btn:hover svg path {
  stroke: #666;
}

@media (max-width: 768px) {
  .notifications-panel {
    width: 90vw;
    right: 5vw;
    left: 5vw;
    top: 70px;
  }
  
  .notifications-panel-header {
    padding: 12px 16px;
  }
  
  .panel-notification-item {
    padding: 10px 16px;
    gap: 10px;
  }
  
  .notification-dot-wrapper {
    width: 6px;
    margin-top: 5px;
  }
  
  .panel-notification-item:not(.unread) .notification-dot-wrapper {
    width: 0;
  }
  
  .panel-notification-item:not(.unread) .panel-notification-content {
    margin-left: -6px;
  }
  
  .notification-dot {
    width: 6px;
    height: 6px;
  }
  
  .panel-notification-header {
    flex-wrap: wrap;
    gap: 6px;
  }
  
  .panel-notification-title {
    white-space: normal;
    line-height: 1.3;
  }
  
  .panel-delete-btn {
    opacity: 1;
    pointer-events: auto;
  }
}
</style>