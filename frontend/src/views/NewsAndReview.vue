<template>
  <section class="news-page">
    <!-- Табы -->
    <nav class="tabs" role="tablist">
      <button
        class="tab"
        :class="{ 'tab--active': activeTab === 'news' }"
        role="tab"
        :aria-selected="activeTab === 'news'"
        @click="activeTab = 'news'"
      >
        Новости
      </button>
      <button
        v-if="isAdmin"
        class="tab"
        :class="{ 'tab--active': activeTab === 'announcements' }"
        role="tab"
        :aria-selected="activeTab === 'announcements'"
        @click="activeTab = 'announcements'"
      >
        Объявления
      </button>
    </nav>

    <!-- Новости -->
    <div v-if="activeTab === 'news'" class="tab-content" role="tabpanel">
      <div class="section-header">
        <h2 class="section-title">Последние новости и обновления</h2>
        <button
          v-if="isAdmin && !showNewsForm"
          class="btn btn--primary"
          @click="openNewsForm()"
        >
          Добавить новость
        </button>
      </div>

      <!-- Inline-форма новости -->
      <div v-if="showNewsForm" class="inline-form">
        <h3 class="inline-form__title">
          {{ editingNews ? 'Редактирование новости' : 'Новая новость' }}
        </h3>
        <div class="form-group">
          <label class="form-label" for="news-title">Заголовок</label>
          <input
            id="news-title"
            type="text"
            class="form-input"
            v-model="newsForm.title"
            placeholder="Заголовок новости"
            maxlength="255"
          >
        </div>
        <div class="form-group">
          <label class="form-label" for="news-description">Краткое описание</label>
          <textarea
            id="news-description"
            class="form-textarea"
            v-model="newsForm.description"
            placeholder="Краткое описание"
            rows="3"
          ></textarea>
        </div>
        <div class="form-group">
          <label class="form-label" for="news-fulltext">Полный текст</label>
          <textarea
            id="news-fulltext"
            class="form-textarea"
            v-model="newsForm.full_text"
            placeholder="Полный текст новости"
            rows="6"
          ></textarea>
        </div>
        <div class="inline-form__actions">
          <button class="btn btn--secondary" @click="showNewsForm = false">Отмена</button>
          <button
            class="btn btn--primary"
            :disabled="!newsForm.title || newsSaving"
            @click="saveNews"
          >
            {{ newsSaving ? 'Сохранение...' : 'Сохранить' }}
          </button>
        </div>
      </div>

      <div v-if="newsLoading" class="loading-state">
        <div class="spinner"></div>
      </div>

      <div v-else-if="newsError" class="error-state">
        <p class="error-message">{{ newsError }}</p>
        <button class="btn btn--primary" @click="fetchNews">Повторить</button>
      </div>

      <div v-else-if="newsList.length === 0" class="empty-state">
        <p class="empty-message">Новостей пока нет</p>
      </div>

      <div v-else class="cards-list">
        <article
          v-for="item in newsList"
          :key="item.id"
          class="card"
        >
          <div class="card__header">
            <time class="card__date">{{ formatDate(item.created_at) }}</time>
            <span
              v-if="isAdmin"
              class="status-badge"
              :class="item.is_active ? 'status-badge--active' : 'status-badge--inactive'"
            >
              {{ item.is_active ? 'Активна' : 'Неактивна' }}
            </span>
          </div>
          <h3 class="card__title">{{ item.title }}</h3>
          <p v-if="item.description" class="card__description">{{ item.description }}</p>

          <div v-if="expandedNewsId === item.id && item.full_text" class="card__full-text">
            {{ item.full_text }}
          </div>

          <div class="card__actions">
            <button
              v-if="item.full_text"
              class="btn btn--link"
              @click="toggleExpand(item.id)"
            >
              {{ expandedNewsId === item.id ? 'Свернуть' : 'Подробнее' }}
            </button>

            <template v-if="isAdmin">
              <button class="btn btn--text" @click="openNewsForm(item)">Редактировать</button>
              <button class="btn btn--text btn--danger" @click="confirmDeleteNews(item)">Удалить</button>
              <button
                class="btn btn--text"
                @click="toggleNewsActive(item)"
              >
                {{ item.is_active ? 'Деактивировать' : 'Активировать' }}
              </button>
            </template>
          </div>
        </article>
      </div>
    </div>

    <!-- Объявления (только админ) -->
    <div v-if="activeTab === 'announcements' && isAdmin" class="tab-content" role="tabpanel">
      <div class="section-header">
        <h2 class="section-title">Объявления</h2>
        <button
          v-if="!showAnnouncementForm"
          class="btn btn--primary"
          @click="openAnnouncementForm()"
        >
          Добавить объявление
        </button>
      </div>

      <!-- Inline-форма объявления -->
      <div v-if="showAnnouncementForm" class="inline-form">
        <h3 class="inline-form__title">
          {{ editingAnnouncement ? 'Редактирование объявления' : 'Новое объявление' }}
        </h3>
        <div class="form-group">
          <label class="form-label" for="ann-title">Заголовок</label>
          <input
            id="ann-title"
            type="text"
            class="form-input"
            v-model="announcementForm.title"
            placeholder="Заголовок объявления"
            maxlength="255"
          >
        </div>
        <div class="form-group">
          <label class="form-label" for="ann-description">Краткое описание</label>
          <textarea
            id="ann-description"
            class="form-textarea"
            v-model="announcementForm.description"
            placeholder="Краткое описание"
            rows="3"
          ></textarea>
        </div>
        <div class="form-group">
          <label class="form-label" for="ann-fulltext">Полный текст</label>
          <textarea
            id="ann-fulltext"
            class="form-textarea"
            v-model="announcementForm.full_text"
            placeholder="Полный текст объявления"
            rows="6"
          ></textarea>
        </div>
        <div class="form-group">
          <label class="checkbox-label">
            <input type="checkbox" v-model="announcementForm.is_important">
            <span class="checkbox-text">Важное объявление</span>
          </label>
        </div>
        <div class="inline-form__actions">
          <button class="btn btn--secondary" @click="showAnnouncementForm = false">Отмена</button>
          <button
            class="btn btn--primary"
            :disabled="!announcementForm.title || announcementSaving"
            @click="saveAnnouncement"
          >
            {{ announcementSaving ? 'Сохранение...' : 'Сохранить' }}
          </button>
        </div>
      </div>

      <div v-if="announcementsLoading" class="loading-state">
        <div class="spinner"></div>
      </div>

      <div v-else-if="announcementsError" class="error-state">
        <p class="error-message">{{ announcementsError }}</p>
        <button class="btn btn--primary" @click="fetchAnnouncements">Повторить</button>
      </div>

      <div v-else-if="announcementsList.length === 0" class="empty-state">
        <p class="empty-message">Объявлений пока нет</p>
      </div>

      <div v-else class="cards-list">
        <article
          v-for="item in announcementsList"
          :key="item.id"
          class="card"
          :class="{ 'card--highlighted': item.is_active }"
        >
          <div class="card__header">
            <time class="card__date">{{ formatDate(item.created_at) }}</time>
            <div class="card__badges">
              <span v-if="item.is_important" class="status-badge status-badge--important">Важное</span>
              <span
                class="status-badge"
                :class="item.is_active ? 'status-badge--active' : 'status-badge--inactive'"
              >
                {{ item.is_active ? 'Активное' : 'Неактивное' }}
              </span>
            </div>
          </div>
          <h3 class="card__title">{{ item.title }}</h3>
          <p v-if="item.description" class="card__description">{{ item.description }}</p>

          <div v-if="expandedAnnouncementId === item.id && item.full_text" class="card__full-text">
            {{ item.full_text }}
          </div>

          <div class="card__actions">
            <button
              v-if="item.full_text"
              class="btn btn--link"
              @click="expandedAnnouncementId = expandedAnnouncementId === item.id ? null : item.id"
            >
              {{ expandedAnnouncementId === item.id ? 'Свернуть' : 'Подробнее' }}
            </button>
            <button
              v-if="!item.is_active"
              class="btn btn--text"
              @click="setActiveAnnouncement(item.id)"
            >
              Сделать активным
            </button>
            <button class="btn btn--text" @click="openAnnouncementForm(item)">Редактировать</button>
            <button class="btn btn--text btn--danger" @click="confirmDeleteAnnouncement(item)">Удалить</button>
          </div>
        </article>
      </div>
    </div>

    <!-- Подтверждение удаления -->
    <ConfirmationModal
      :show="showDeleteConfirm"
      title="Подтверждение удаления"
      :message="deleteConfirmMessage"
      confirm-text="Удалить"
      :confirm-button-style="{ background: '#dc3545', borderColor: '#dc3545' }"
      @confirm="executeDelete"
      @cancel="showDeleteConfirm = false"
    />

    <!-- Toast -->
    <transition name="toast-fade">
      <div v-if="toast.visible" class="toast" :class="'toast--' + toast.type">
        {{ toast.message }}
      </div>
    </transition>
  </section>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import ConfirmationModal from '@/components/ConfirmationModal.vue'

export default {
  name: 'NewsAndReview',
  components: {
    ConfirmationModal,
  },
  data() {
    return {
      activeTab: 'news',

      // News
      newsList: [],
      newsLoading: false,
      newsError: null,
      showNewsForm: false,
      editingNews: null,
      newsForm: { title: '', description: '', full_text: '' },
      newsSaving: false,
      expandedNewsId: null,

      // Announcements
      announcementsList: [],
      announcementsLoading: false,
      announcementsError: null,
      showAnnouncementForm: false,
      editingAnnouncement: null,
      announcementForm: { title: '', description: '', full_text: '', is_important: false },
      announcementSaving: false,
      expandedAnnouncementId: null,

      // Delete confirmation
      showDeleteConfirm: false,
      deleteConfirmMessage: '',
      pendingDelete: null,

      // Toast
      toast: { visible: false, message: '', type: 'success', timer: null },
    }
  },
  computed: {
    isAdmin() {
      const authStore = useAuthStore()
      const t = authStore.userType
      return t === 5 || t === 6
    },
  },
  watch: {
    activeTab(tab) {
      if (tab === 'announcements' && this.announcementsList.length === 0) {
        this.fetchAnnouncements()
      }
    },
  },
  methods: {
    async fetchNews() {
      this.newsLoading = true
      this.newsError = null
      try {
        const endpoint = this.isAdmin ? '/news/all' : '/news'
        const response = await apiRequest(endpoint)
        if (response.ok) {
          this.newsList = await response.json() || []
        } else {
          const err = await response.json()
          this.newsError = err.message || 'Не удалось загрузить новости'
        }
      } catch {
        this.newsError = 'Ошибка сети при загрузке новостей'
      } finally {
        this.newsLoading = false
      }
    },

    async fetchAnnouncements() {
      this.announcementsLoading = true
      this.announcementsError = null
      try {
        const response = await apiRequest('/announcements/all')
        if (response.ok) {
          this.announcementsList = await response.json() || []
        } else {
          const err = await response.json()
          this.announcementsError = err.message || 'Не удалось загрузить объявления'
        }
      } catch {
        this.announcementsError = 'Ошибка сети при загрузке объявлений'
      } finally {
        this.announcementsLoading = false
      }
    },

    openNewsForm(item = null) {
      this.editingNews = item
      this.newsForm = item
        ? { title: item.title, description: item.description || '', full_text: item.full_text || '' }
        : { title: '', description: '', full_text: '' }
      this.showNewsForm = true
    },

    async saveNews() {
      this.newsSaving = true
      try {
        const body = {
          title: this.newsForm.title,
          description: this.newsForm.description || null,
          full_text: this.newsForm.full_text || null,
        }

        let response
        if (this.editingNews) {
          response = await apiRequest(`/news/${this.editingNews.id}`, {
            method: 'PUT',
            body: JSON.stringify(body),
          })
        } else {
          response = await apiRequest('/news', {
            method: 'POST',
            body: JSON.stringify(body),
          })
        }

        if (response.ok) {
          this.showNewsForm = false
          this.showToast(this.editingNews ? 'Новость обновлена' : 'Новость создана', 'success')
          await this.fetchNews()
        } else {
          const err = await response.json()
          this.showToast(err.message || 'Ошибка сохранения', 'error')
        }
      } catch {
        this.showToast('Ошибка сети', 'error')
      } finally {
        this.newsSaving = false
      }
    },

    async toggleNewsActive(item) {
      try {
        const response = await apiRequest(`/news/${item.id}`, {
          method: 'PUT',
          body: JSON.stringify({ is_active: !item.is_active }),
        })
        if (response.ok) {
          this.showToast(item.is_active ? 'Новость деактивирована' : 'Новость активирована', 'success')
          await this.fetchNews()
        } else {
          const err = await response.json()
          this.showToast(err.message || 'Ошибка обновления', 'error')
        }
      } catch {
        this.showToast('Ошибка сети', 'error')
      }
    },

    confirmDeleteNews(item) {
      this.pendingDelete = { type: 'news', id: item.id }
      this.deleteConfirmMessage = `Удалить новость "${item.title}"?`
      this.showDeleteConfirm = true
    },

    openAnnouncementForm(item = null) {
      this.editingAnnouncement = item
      this.announcementForm = item
        ? {
            title: item.title,
            description: item.description || '',
            full_text: item.full_text || '',
            is_important: item.is_important || false,
          }
        : { title: '', description: '', full_text: '', is_important: false }
      this.showAnnouncementForm = true
    },

    async saveAnnouncement() {
      this.announcementSaving = true
      try {
        const body = {
          title: this.announcementForm.title,
          description: this.announcementForm.description || null,
          full_text: this.announcementForm.full_text || null,
          is_important: this.announcementForm.is_important,
        }

        let response
        if (this.editingAnnouncement) {
          response = await apiRequest(`/announcements/${this.editingAnnouncement.id}`, {
            method: 'PUT',
            body: JSON.stringify(body),
          })
        } else {
          response = await apiRequest('/announcements', {
            method: 'POST',
            body: JSON.stringify(body),
          })
        }

        if (response.ok) {
          this.showAnnouncementForm = false
          this.showToast(
            this.editingAnnouncement ? 'Объявление обновлено' : 'Объявление создано',
            'success',
          )
          await this.fetchAnnouncements()
        } else {
          const err = await response.json()
          this.showToast(err.message || 'Ошибка сохранения', 'error')
        }
      } catch {
        this.showToast('Ошибка сети', 'error')
      } finally {
        this.announcementSaving = false
      }
    },

    async setActiveAnnouncement(id) {
      try {
        const response = await apiRequest('/announcements/set-active', {
          method: 'POST',
          body: JSON.stringify({ announcement_id: id }),
        })
        if (response.ok) {
          this.showToast('Активное объявление обновлено', 'success')
          await this.fetchAnnouncements()
        } else {
          const err = await response.json()
          this.showToast(err.message || 'Ошибка обновления', 'error')
        }
      } catch {
        this.showToast('Ошибка сети', 'error')
      }
    },

    confirmDeleteAnnouncement(item) {
      this.pendingDelete = { type: 'announcement', id: item.id }
      this.deleteConfirmMessage = `Удалить объявление "${item.title}"?`
      this.showDeleteConfirm = true
    },

    async executeDelete() {
      this.showDeleteConfirm = false
      if (!this.pendingDelete) return

      const { type, id } = this.pendingDelete
      this.pendingDelete = null

      try {
        const endpoint = type === 'news' ? `/news/${id}` : `/announcements/${id}`
        const response = await apiRequest(endpoint, { method: 'DELETE' })
        if (response.ok) {
          this.showToast(type === 'news' ? 'Новость удалена' : 'Объявление удалено', 'success')
          if (type === 'news') {
            await this.fetchNews()
          } else {
            await this.fetchAnnouncements()
          }
        } else {
          const err = await response.json()
          this.showToast(err.message || 'Ошибка удаления', 'error')
        }
      } catch {
        this.showToast('Ошибка сети', 'error')
      }
    },

    toggleExpand(id) {
      this.expandedNewsId = this.expandedNewsId === id ? null : id
    },

    formatDate(dateString) {
      if (!dateString) return ''
      return new Date(dateString).toLocaleDateString('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      }).replace(',', '')
    },

    showToast(message, type) {
      if (this.toast.timer) clearTimeout(this.toast.timer)
      this.toast.message = message
      this.toast.type = type
      this.toast.visible = true
      this.toast.timer = setTimeout(() => {
        this.toast.visible = false
      }, 3000)
    },
  },
  mounted() {
    this.fetchNews()
    if (this.isAdmin) {
      this.fetchAnnouncements()
    }
  },
  beforeUnmount() {
    if (this.toast.timer) clearTimeout(this.toast.timer)
  },
}
</script>

<style scoped>
.news-page {
  padding: 12px;
}

/* Табы */
.tabs {
  display: flex;
  gap: 4px;
  margin-bottom: 16px;
}

.tab {
  padding: 8px 20px;
  border: 1px solid var(--color-border);
  border-radius: 50px;
  background: #fff;
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text);
  cursor: pointer;
  transition: all 0.2s;
  font-family: 'Montserrat', sans-serif;
}

.tab:hover {
  background: #f5f5f5;
}

.tab--active {
  background: var(--color-primary);
  color: #fff;
  border-color: var(--color-primary);
}

.tab--active:hover {
  background: var(--color-primary-hover);
}

/* Секция */
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  color: #000;
  margin: 0;
}

/* Inline-форма */
.inline-form {
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: 20px;
  padding: 20px;
  margin-bottom: 16px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
}

.inline-form__title {
  margin: 0 0 16px;
  font-size: 15px;
  font-weight: 600;
  color: var(--color-primary);
}

.inline-form__actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 4px;
}

/* Карточки */
.cards-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.card {
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: 20px;
  padding: 16px 20px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
  transition: box-shadow 0.2s;
}

.card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
}

.card--highlighted {
  border-color: var(--color-primary);
  background: #f8f8ff;
}

.card__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.card__date {
  font-size: 12px;
  font-weight: 500;
  color: #a2a2a2;
}

.card__badges {
  display: flex;
  gap: 6px;
}

.card__title {
  margin: 0 0 8px;
  font-size: 15px;
  font-weight: 600;
  color: var(--color-primary);
}

.card__description {
  margin: 0 0 8px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--color-text);
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card__full-text {
  margin: 8px 0 12px;
  padding-top: 8px;
  border-top: 1px solid var(--color-border);
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-text);
  white-space: pre-wrap;
}

.card__actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 8px;
}

/* Бейджи статуса */
.status-badge {
  display: inline-block;
  padding: 2px 10px;
  border-radius: 50px;
  font-size: 11px;
  font-weight: 600;
}

.status-badge--active {
  background: #e6f9ed;
  color: #1a7a3a;
}

.status-badge--inactive {
  background: #f2f2f2;
  color: #888;
}

.status-badge--important {
  background: #fde8e8;
  color: var(--color-danger);
}

/* Кнопки */
.btn {
  padding: 6px 16px;
  border: none;
  border-radius: 50px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  font-family: 'Montserrat', sans-serif;
  transition: all 0.2s;
  display: inline-flex;
  align-items: center;
}

.btn--primary {
  background: var(--color-primary);
  color: #fff;
  height: 32px;
  padding: 0 20px;
}

.btn--primary:hover:not(:disabled) {
  background: var(--color-primary-hover);
}

.btn--primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn--secondary {
  background: #fff;
  color: var(--color-text);
  border: 1px solid var(--color-border);
  height: 32px;
  padding: 0 20px;
}

.btn--secondary:hover {
  background: #f5f5f5;
}

.btn--link {
  background: var(--color-primary);
  color: #fff;
  padding: 4px 14px;
  font-size: 12px;
  border-radius: 50px;
}

.btn--link:hover {
  background: var(--color-primary-hover);
}

.btn--text {
  background: none;
  color: var(--color-primary);
  padding: 4px 8px;
  font-size: 12px;
}

.btn--text:hover {
  text-decoration: underline;
}

.btn--danger {
  color: var(--color-danger);
}

/* Форма */
.form-group {
  margin-bottom: 14px;
}

.form-label {
  display: block;
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text);
  margin-bottom: 6px;
  font-family: 'Montserrat', sans-serif;
}

.form-input {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  font-size: 13px;
  font-family: 'Montserrat', sans-serif;
  color: var(--color-text);
  transition: border-color 0.2s;
  outline: none;
}

.form-input:focus {
  border-color: var(--color-primary);
}

.form-textarea {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  font-size: 13px;
  font-family: 'Montserrat', sans-serif;
  color: var(--color-text);
  transition: border-color 0.2s;
  outline: none;
  resize: vertical;
  min-height: 60px;
}

.form-textarea:focus {
  border-color: var(--color-primary);
}

.checkbox-label {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  font-size: 13px;
}

.checkbox-label input[type="checkbox"] {
  accent-color: var(--color-primary);
  cursor: pointer;
}

.checkbox-text {
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  color: var(--color-text);
}

/* Состояния */
.loading-state {
  display: flex;
  justify-content: center;
  padding: 40px;
}

.spinner {
  width: 24px;
  height: 24px;
  border: 2px solid var(--color-border);
  border-top-color: var(--color-primary);
  border-radius: 50%;
  animation: spinner-rotate 1s linear infinite;
}

@keyframes spinner-rotate {
  to { transform: rotate(360deg); }
}

.error-state {
  text-align: center;
  padding: 40px 20px;
}

.error-message {
  color: var(--color-danger);
  font-size: 13px;
  margin: 0 0 12px;
}

.empty-state {
  text-align: center;
  padding: 40px;
}

.empty-message {
  color: #a2a2a2;
  font-size: 14px;
}

/* Toast */
.toast {
  position: fixed;
  bottom: 24px;
  right: 24px;
  padding: 10px 20px;
  border-radius: 12px;
  font-size: 13px;
  font-weight: 500;
  font-family: 'Montserrat', sans-serif;
  color: #fff;
  z-index: 9999;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.toast--success {
  background: var(--color-success);
}

.toast--error {
  background: var(--color-danger);
}

.toast-fade-enter-active,
.toast-fade-leave-active {
  transition: all 0.3s ease;
}

.toast-fade-enter-from,
.toast-fade-leave-to {
  opacity: 0;
  transform: translateY(10px);
}

/* Адаптивность */
@media (max-width: 768px) {
  .section-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }

  .card {
    padding: 12px 14px;
    border-radius: 16px;
  }

  .card__actions {
    gap: 4px;
  }

  .inline-form {
    padding: 14px;
    border-radius: 16px;
  }
}
</style>
