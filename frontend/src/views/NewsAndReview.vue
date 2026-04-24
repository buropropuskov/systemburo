<template>
  <section class="news">
    <div class="content-wrapper">
      <!-- Левая колонка - Новости -->
      <div class="left-column">
        <div class="news-container">
          <div class="news-header">
            <h2 class="news-title">
              Последние новости
            </h2>
            <div class="header-actions">
              <button
                class="manage-btn"
                @click="openManageModal"
              >
                Управление
              </button>
              <RefreshButton @refresh="fetchAllData" />
            </div>
          </div>
          <div class="divider" />
          <div class="news-list">
            <!-- Лоадер -->
            <div
              v-if="loadingNews"
              class="loading-message"
            >
              <LoaderSpinner label="Загрузка новостей…" />
            </div>
            <!-- Новости -->
            <div
              v-else-if="newsItems.length > 0"
              class="news-items"
            >
              <article
                v-for="(item, index) in newsItems"
                :key="item.id"
                class="news-item"
                :style="{ animationDelay: `${index * 0.1}s` }"
              >
                <div class="item-header">
                  <time class="news-date">{{ formatDate(item.created_at) }}</time>
                  <span class="item-type">Новость</span>
                </div>
                <h3 class="news-item-title">
                  {{ item.title }}
                </h3>
                <p class="news-item-description">
                  {{ item.description }}
                </p>
                <button
                  class="news-details-button"
                  @click="openNewsModal(item)"
                >
                  Читать далее
                </button>
              </article>
            </div>
            <div
              v-else
              class="empty-state"
            >
              <p>Нет новостей</p>
            </div>
          </div>
        </div>
      </div>

      <!-- Правая колонка -->
      <div class="right-column">
        <div
          class="guide-card"
          :style="{ animationDelay: '0.2s' }"
          @click="openGuide"
        >
          <div class="guide-content">
            <div class="guide-text">
              <h3 class="guide-title">
                <div class="guide-icon">
                  <svg
                    width="35"
                    height="35"
                    viewBox="0 0 35 35"
                    fill="none"
                    xmlns="http://www.w3.org/2000/svg"
                  >
                    <rect
                      width="35"
                      height="35"
                      fill="url(#pattern0)"
                    />
                    <defs>
                      <pattern
                        id="pattern0"
                        patternContentUnits="objectBoundingBox"
                        width="1"
                        height="1"
                      >
                        <use
                          xlink:href="#image0"
                          transform="scale(0.015625)"
                        />
                      </pattern>
                      <image
                        id="image0"
                        width="64"
                        height="64"
                        preserveAspectRatio="none"
                        xlink:href="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAEAAAABACAYAAACqaXHeAAAACXBIWXMAAAHYAAAB2AH6XKZyAAAAGXRFWHRTb2Z0d2FyZQB3d3cuaW5rc2NhcGUub3Jnm+48GgAAAwxJREFUeJzt209oHkUcxvGPMQlqBAk9WCoBJZdoDuLJQyMtlBZSKv4Be6m3QpRcKmoN9RQPpVCCUA+e9FRPCqWlHixeVGwPUpFCrNhi0ItQJH+MQpU2TQ+7L+8SeHl3N7OZTbJfWN7Z3fnNPPMw876zs/PSsL15oGRcP17AM3gknJyg/IwvQxfajynMY3UTHM+HbPwgvqtBo4ocr3drVG/OxvfiC0m3bzGHS1jOWcZaxrA7TV/G94FiDmG0pKaOvKHt6grexYPrLHM6U+Z0wJizKugBxzPpDzCTM6729OTI8zSG0/QiTlcnZ+PJY8BwJn0F/1WkJQp5DHgsk16qSkgs8hiQnSytViUkFnkM2NI0BsQWEJvGgNgCOjCDf2zAhKuOBkzhHTyafr5XZWV1M+AgTq65dgovV1VhnQx4Dp9rP2TdTT978Fl6Pzh1MWAnLmAgPf8dz+K39HxAsrrzROiK62LABIbS9DJexHVJ12+tN+zC0dAV18WA1nR7RfIMP5uez+Kw9nAou4bZkboY0OJtXFxz7ZJkAaYS6mTAp/iow70z+LiKSmMakH20/hpvdsl/LM3XYjGEiLxLYlXwCZ5EH97XHueduIvXJPOC/9P4dRPTgH/xVsGYvzEZUkSdvgOi0BgQW0BsGgNiC4hNY0BsAbFpDIgtIDaNAbEFxKYxILaA2DQGxBYQm8aA2AJis+0NKLskNoZXJRsSd4STE4SnimQuasAQvsGegnGxWOmWoagBm6XhJMvuV7plKjsE/pK8qLiMhZJlVM2cHO8OyhjwE8Zxq0Rs7Sj6K3Abr9gijad4DziHP9L0wziBkaCKwnAPV/Fhml4XR7S3n2f360yq5k8OIY8D3RqXZwhkX2I+lEn/mSM2JityDNU8Gw5G8Eua/hZ7M/d2S3Zu1JEbuBaqsJva3Wo8VKGbiQltA5bwUlw54ci756YXX2Ff5tqPkn+RLeBOYF0h+BXnQxY4KHkOiP3NXuQY69aoIhOhReyX/BTOF4irNWW3nfVJ3B3F4+l5nbiHHwQeAg1bkfsR5B+/y7yHCAAAAABJRU5ErkJggg=="
                      />
                    </defs>
                  </svg>
                </div>
                <div class="guide-title-text">
                  Руководство пользования системой <span class="guide-title-blue">подачи заявок</span>
                </div>
              </h3>
              <p class="guide-description">
                Узнайте, как пользоваться системой, легко и быстро подать заявку:
                <strong>пошаговая инструкция</strong>
              </p>
            </div>
          </div>
        </div>
        <div
          v-if="activeAnnouncement"
          class="news-card announcement-card"
          :class="{ 'important-announcement': activeAnnouncement.is_important }"
          :style="{ animationDelay: '0.1s' }"
          @click="openAnnouncementModal(activeAnnouncement)"
        >
          <div class="card-header">
            <span
              class="card-type"
              :class="{ important: activeAnnouncement.is_important }"
            >
              {{ activeAnnouncement.is_important ? 'Важное объявление' : 'Объявление' }}
            </span>
            <time class="card-date">{{ formatDate(activeAnnouncement.created_at) }}</time>
          </div>
          <h3 class="card-title">
            {{ activeAnnouncement.title }}
          </h3>
          <p class="card-description">
            {{ activeAnnouncement.description }}
          </p>
          <button class="card-button">
            Читать далее
          </button>
        </div>
      </div>
    </div>

    <!-- Модальное окно управления -->
    <transition name="modal-fade">
      <div
        v-if="showManageModal"
        class="modal-overlay"
        @click.self="closeManageModal"
      >
        <div class="modal-content manage-modal">
          <div class="modal-header">
            <h3 class="modal-title">
              Управление контентом
            </h3>
            <button
              class="modal-close"
              @click="closeManageModal"
            >
              <svg
                width="10"
                height="10"
                viewBox="0 0 14 14"
                fill="none"
              ><path
                d="M13 1L1 13M1 1L13 13"
                stroke="#666"
                stroke-width="2"
                stroke-linecap="round"
              /></svg>
            </button>
          </div>

          <div class="modal-body">
            <div class="manage-tabs">
              <button
                class="tab-btn"
                :class="{ active: activeTab === 'news' }"
                @click="activeTab = 'news'"
              >
                Новости
              </button>
              <button
                class="tab-btn"
                :class="{ active: activeTab === 'announcements' }"
                @click="activeTab = 'announcements'"
              >
                Объявления
              </button>
            </div>

            <!-- Новости -->
            <div
              v-if="activeTab === 'news'"
              class="manage-section"
            >
              <div class="section-header">
                <div class="section-title">
                  Список новостей
                </div>
                <button
                  class="add-btn"
                  @click="openCreateNewsModal"
                >
                  + Добавить новость
                </button>
              </div>
              <div class="items-list">
                <div
                  v-for="item in allNewsItems"
                  :key="item.id"
                  class="manage-item"
                >
                  <div class="item-main">
                    <div class="item-title">
                      {{ item.title }}
                    </div>
                    <div class="item-meta">
                      <span>{{ formatDate(item.created_at) }}</span>
                      <span>{{ item.created_by_name || 'Система' }}</span>
                      <span class="status-text">{{ item.is_active ? 'Активна' : 'Скрыта' }}</span>
                    </div>
                  </div>
                  <div class="item-actions">
                    <button
                      class="action-btn edit-btn"
                      @click="editNewsItem(item)"
                    >
                      Редактировать
                    </button>
                    <button
                      class="action-btn toggle-btn"
                      @click="toggleNewsActive(item)"
                    >
                      {{ item.is_active ? 'Скрыть' : 'Показать' }}
                    </button>
                    <button
                      class="action-btn delete-btn"
                      @click="deleteNewsItem(item.id)"
                    >
                      Удалить
                    </button>
                  </div>
                </div>
                <div
                  v-if="allNewsItems.length === 0"
                  class="empty-manage"
                >
                  Нет новостей
                </div>
              </div>
            </div>

            <!-- Объявления -->
            <div
              v-if="activeTab === 'announcements'"
              class="manage-section"
            >
              <div class="section-header">
                <div class="section-title">
                  Список объявлений
                </div>
                <button
                  class="add-btn"
                  @click="openCreateAnnouncementModal"
                >
                  + Добавить объявление
                </button>
              </div>
              <div class="info-message">
                <span>Активно может быть только одно объявление</span>
              </div>
              <div class="items-list">
                <div
                  v-for="item in allAnnouncements"
                  :key="item.id"
                  class="manage-item"
                >
                  <div class="item-main">
                    <div class="item-title">
                      {{ item.title }}
                      <span
                        v-if="item.is_important"
                        class="important-badge"
                      >Важное</span>
                      <span
                        v-if="item.is_active"
                        class="active-badge"
                      >Активно</span>
                    </div>
                    <div class="item-meta">
                      <span>{{ formatDate(item.created_at) }}</span>
                      <span>Создал: {{ item.created_by_name || 'Система' }}</span>
                      <span v-if="item.activated_by_name">Активировал: {{ item.activated_by_name }}</span>
                    </div>
                  </div>
                  <div class="item-actions">
                    <button
                      class="action-btn edit-btn"
                      @click="editAnnouncement(item)"
                    >
                      Редактировать
                    </button>
                    <button
                      v-if="!item.is_active"
                      class="action-btn activate-btn"
                      @click="setActiveAnnouncement(item.id)"
                    >
                      Активировать
                    </button>
                    <button
                      v-if="item.is_active"
                      class="action-btn deactivate-btn"
                      @click="deactivateAnnouncement(item.id)"
                    >
                      Деактивировать
                    </button>
                    <button
                      class="action-btn delete-btn"
                      @click="deleteAnnouncement(item.id)"
                    >
                      Удалить
                    </button>
                  </div>
                </div>
                <div
                  v-if="allAnnouncements.length === 0"
                  class="empty-manage"
                >
                  Нет объявлений
                </div>
              </div>
            </div>
          </div>

          <div class="modal-footer">
            <button
              class="btn close-btn"
              @click="closeManageModal"
            >
              Закрыть
            </button>
          </div>
        </div>
      </div>
    </transition>

    <!-- Модальное окно создания/редактирования новости -->
    <transition name="modal-fade">
      <div
        v-if="showNewsModal"
        class="modal-overlay"
        @click.self="closeNewsModal"
      >
        <div class="modal-content">
          <div class="modal-header">
            <h3 class="modal-title">
              {{ editingNews ? 'Редактировать новость' : 'Создать новость' }}
            </h3>
            <button
              class="modal-close"
              @click="closeNewsModal"
            >
              <svg
                width="10"
                height="10"
                viewBox="0 0 14 14"
                fill="none"
              ><path
                d="M13 1L1 13M1 1L13 13"
                stroke="#666"
                stroke-width="2"
                stroke-linecap="round"
              /></svg>
            </button>
          </div>
          <div class="modal-body">
            <div class="form-group">
              <label class="form-label">Заголовок</label><input
                v-model="newsForm.title"
                type="text"
                class="form-input"
                placeholder="Введите заголовок"
              >
            </div>
            <div class="form-group">
              <label class="form-label">Краткое описание</label><textarea
                v-model="newsForm.description"
                class="form-textarea"
                placeholder="Введите краткое описание"
                rows="3"
              />
            </div>
            <div class="form-group">
              <label class="form-label">Полный текст</label><textarea
                v-model="newsForm.fullText"
                class="form-textarea"
                placeholder="Введите полный текст"
                rows="5"
              />
            </div>
          </div>
          <div class="modal-footer">
            <button
              class="btn cancel-btn"
              @click="closeNewsModal"
            >
              Отмена
            </button>
            <button
              class="btn save-btn"
              @click="submitNews"
            >
              Сохранить
            </button>
          </div>
        </div>
      </div>
    </transition>

    <!-- Модальное окно создания/редактирования объявления -->
    <transition name="modal-fade">
      <div
        v-if="showAnnouncementModal"
        class="modal-overlay"
        @click.self="closeAnnouncementModal"
      >
        <div class="modal-content">
          <div class="modal-header">
            <h3 class="modal-title">
              {{ editingAnnouncement ? 'Редактировать объявление' : 'Создать объявление' }}
            </h3>
            <button
              class="modal-close"
              @click="closeAnnouncementModal"
            >
              <svg
                width="10"
                height="10"
                viewBox="0 0 14 14"
                fill="none"
              ><path
                d="M13 1L1 13M1 1L13 13"
                stroke="#666"
                stroke-width="2"
                stroke-linecap="round"
              /></svg>
            </button>
          </div>
          <div class="modal-body">
            <div class="form-group">
              <label class="form-label">Заголовок</label><input
                v-model="announcementForm.title"
                type="text"
                class="form-input"
                placeholder="Введите заголовок"
              >
            </div>
            <div class="form-group">
              <label class="form-label">Краткое описание</label><textarea
                v-model="announcementForm.description"
                class="form-textarea"
                placeholder="Введите краткое описание"
                rows="3"
              />
            </div>
            <div class="form-group">
              <label class="form-label">Полный текст</label><textarea
                v-model="announcementForm.fullText"
                class="form-textarea"
                placeholder="Введите полный текст"
                rows="5"
              />
            </div>
            <div class="form-group">
              <label class="checkbox-label"><input
                v-model="announcementForm.isImportant"
                type="checkbox"
              ><span>Важное объявление</span></label>
            </div>
          </div>
          <div class="modal-footer">
            <button
              class="btn cancel-btn"
              @click="closeAnnouncementModal"
            >
              Отмена
            </button>
            <button
              class="btn save-btn"
              @click="submitAnnouncement"
            >
              Сохранить
            </button>
          </div>
        </div>
      </div>
    </transition>

    <!-- Модальное окно просмотра новости -->
    <transition name="modal-fade">
      <div
        v-if="showNewsDetailsModal"
        class="modal-overlay"
        @click.self="closeNewsDetailsModal"
      >
        <div class="modal-content">
          <div class="modal-body">
            <div class="modal-info">
              <time class="modal-date">{{ formatDate(selectedNews?.created_at) }}</time><span class="modal-type">Новость</span>
            </div>
            <h3 class="modal-title">
              {{ selectedNews?.title }}
            </h3>
            <p class="modal-description">
              {{ selectedNews?.description }}
            </p>
            <div
              v-if="selectedNews?.full_text"
              class="modal-full-text"
            >
              {{ selectedNews.full_text }}
            </div>
          </div>
          <div class="modal-footer">
            <button
              class="btn close-btn"
              @click="closeNewsDetailsModal"
            >
              Закрыть
            </button>
          </div>
        </div>
      </div>
    </transition>

    <!-- Модальное окно просмотра объявления - используем AnnouncementModal -->
    <AnnouncementModal 
      :show="showViewAnnouncementModal"
      :announcement="viewingAnnouncement"
      @close="closeViewAnnouncementModal"
    />
  </section>
</template>

<script>
import { apiRequest } from '@/api/client'
import RefreshButton from '../components/RefreshButton.vue'
import AnnouncementModal from '../components/AnnouncementModal.vue'
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue'

export default {
  name: 'LatestNews',
  components: {
    RefreshButton,
    AnnouncementModal,
    LoaderSpinner
  },
  data() {
    return {
      loadingNews: false,
      showManageModal: false,
      showNewsDetailsModal: false,
      showNewsModal: false,
      showAnnouncementModal: false,
      showViewAnnouncementModal: false,
      activeTab: 'news',
      selectedNews: null,
      viewingAnnouncement: null,
      editingNews: null,
      editingAnnouncement: null,
      newsForm: { title: '', description: '', fullText: '' },
      announcementForm: { title: '', description: '', fullText: '', isImportant: false },
      newsItems: [],
      allNewsItems: [],
      activeAnnouncement: null,
      allAnnouncements: []
    }
  },
  mounted() {
    this.fetchAllData()
  },
  methods: {
    formatDate(dateString) {
      if (!dateString) return ''
      const date = new Date(dateString)
      // Добавляем 3 часа для UTC+3 (Москва)
      const mskDate = new Date(date.getTime() + 3 * 60 * 60 * 1000)
      return mskDate.toLocaleDateString('ru-RU', { 
        day: '2-digit', 
        month: '2-digit', 
        year: 'numeric', 
        hour: '2-digit', 
        minute: '2-digit' 
      }).replace(',', '')
    },

    async fetchNews() {
      this.loadingNews = true
      try {
        const response = await apiRequest('/news')
        if (response.ok) this.newsItems = await response.json()
      } catch (error) { console.error('Ошибка загрузки новостей:', error) }
      finally { this.loadingNews = false }
    },
    async fetchAllNews() {
      try {
        const response = await apiRequest('/news/all')
        if (response.ok) this.allNewsItems = await response.json()
      } catch (error) { console.error('Ошибка загрузки всех новостей:', error) }
    },
    async fetchActiveAnnouncement() {
      try {
        const response = await apiRequest('/announcements/active')
        if (response.ok) {
          this.activeAnnouncement = await response.json()
        } else {
          this.activeAnnouncement = null
        }
      } catch (error) { console.error('Ошибка загрузки активного объявления:', error) }
    },
    async fetchAllAnnouncements() {
      try {
        const response = await apiRequest('/announcements/all')
        if (response.ok) this.allAnnouncements = await response.json()
      } catch (error) { console.error('Ошибка загрузки всех объявлений:', error) }
    },
    async fetchAllData() {
      await Promise.all([this.fetchNews(), this.fetchActiveAnnouncement()])
    },

    buildNewsPayload() {
      return {
        title: this.newsForm.title,
        description: this.newsForm.description,
        full_text: this.newsForm.fullText,
      }
    },
    async createNews() {
      try {
        const response = await apiRequest('/news', {
          method: 'POST',
          body: JSON.stringify(this.buildNewsPayload())
        })
        if (response.ok) {
          await this.fetchAllNews()
          await this.fetchNews()
          this.closeNewsModal()
        } else alert('Ошибка создания новости')
      } catch (error) { console.error('Ошибка создания новости:', error) }
    },
    async updateNews() {
      try {
        const response = await apiRequest(`/news/${this.editingNews.id}`, {
          method: 'PUT',
          body: JSON.stringify(this.buildNewsPayload())
        })
        if (response.ok) {
          await this.fetchAllNews()
          await this.fetchNews()
          this.closeNewsModal()
        } else alert('Ошибка обновления новости')
      } catch (error) { console.error('Ошибка обновления новости:', error) }
    },
    async toggleNewsActive(item) {
      try {
        const response = await apiRequest(`/news/${item.id}`, {
          method: 'PUT',
          body: JSON.stringify({ is_active: !item.is_active })
        })
        if (response.ok) {
          await this.fetchAllNews()
          await this.fetchNews()
        } else alert('Ошибка изменения статуса')
      } catch (error) { console.error('Ошибка изменения статуса:', error) }
    },
    async deleteNewsItem(id) {
      if (!confirm('Удалить новость?')) return
      try {
        const response = await apiRequest(`/news/${id}`, { method: 'DELETE' })
        if (response.ok) {
          await this.fetchAllNews()
          await this.fetchNews()
        } else alert('Ошибка удаления новости')
      } catch (error) { console.error('Ошибка удаления новости:', error) }
    },

    buildAnnouncementPayload() {
      return {
        title: this.announcementForm.title,
        description: this.announcementForm.description,
        full_text: this.announcementForm.fullText,
        is_important: this.announcementForm.isImportant,
      }
    },
    async createAnnouncement() {
      try {
        const response = await apiRequest('/announcements', {
          method: 'POST',
          body: JSON.stringify(this.buildAnnouncementPayload())
        })
        if (response.ok) {
          await this.fetchAllAnnouncements()
          await this.fetchActiveAnnouncement()
          this.closeAnnouncementModal()
        } else alert('Ошибка создания объявления')
      } catch (error) { console.error('Ошибка создания объявления:', error) }
    },
    async updateAnnouncement() {
      try {
        const response = await apiRequest(`/announcements/${this.editingAnnouncement.id}`, {
          method: 'PUT',
          body: JSON.stringify(this.buildAnnouncementPayload())
        })
        if (response.ok) {
          await this.fetchAllAnnouncements()
          await this.fetchActiveAnnouncement()
          this.closeAnnouncementModal()
        } else alert('Ошибка обновления объявления')
      } catch (error) { console.error('Ошибка обновления объявления:', error) }
    },
    async setActiveAnnouncement(id) {
      if (!confirm('Активировать это объявление?')) return
      try {
        const response = await apiRequest('/announcements/set-active', {
          method: 'POST',
          body: JSON.stringify({ announcement_id: id })
        })
        if (response.ok) {
          await this.fetchAllAnnouncements()
          await this.fetchActiveAnnouncement()
        } else alert('Ошибка активации объявления')
      } catch (error) { console.error('Ошибка активации объявления:', error) }
    },
    async deactivateAnnouncement(id) {
      if (!confirm('Деактивировать объявление?')) return
      try {
        const response = await apiRequest(`/announcements/${id}`, {
          method: 'PUT',
          body: JSON.stringify({ is_active: false })
        })
        if (response.ok) {
          await this.fetchAllAnnouncements()
          await this.fetchActiveAnnouncement()
        } else alert('Ошибка деактивации объявления')
      } catch (error) { console.error('Ошибка деактивации объявления:', error) }
    },
    async deleteAnnouncement(id) {
      if (!confirm('Удалить объявление?')) return
      try {
        const response = await apiRequest(`/announcements/${id}`, { method: 'DELETE' })
        if (response.ok) {
          await this.fetchAllAnnouncements()
          await this.fetchActiveAnnouncement()
        } else alert('Ошибка удаления объявления')
      } catch (error) { console.error('Ошибка удаления объявления:', error) }
    },

    openManageModal() { this.fetchAllNews(); this.fetchAllAnnouncements(); this.showManageModal = true },
    closeManageModal() { this.showManageModal = false },
    openNewsModal(item) { this.selectedNews = item; this.showNewsDetailsModal = true },
    closeNewsDetailsModal() { this.showNewsDetailsModal = false; this.selectedNews = null },
    openAnnouncementModal(announcement) { 
      this.viewingAnnouncement = announcement; 
      this.showViewAnnouncementModal = true; 
    },
    closeViewAnnouncementModal() { 
      this.showViewAnnouncementModal = false; 
      this.viewingAnnouncement = null; 
    },
    openCreateNewsModal() { this.editingNews = null; this.newsForm = { title: '', description: '', fullText: '' }; this.showNewsModal = true },
    editNewsItem(item) { this.editingNews = item; this.newsForm = { title: item.title, description: item.description, fullText: item.full_text || '' }; this.showNewsModal = true },
    closeNewsModal() { this.showNewsModal = false; this.editingNews = null },
    submitNews() { if (!this.newsForm.title || !this.newsForm.description) { alert('Заполните заголовок и описание'); return } this.editingNews ? this.updateNews() : this.createNews() },
    openCreateAnnouncementModal() { this.editingAnnouncement = null; this.announcementForm = { title: '', description: '', fullText: '', isImportant: false }; this.showAnnouncementModal = true },
    editAnnouncement(item) { this.editingAnnouncement = item; this.announcementForm = { title: item.title, description: item.description, fullText: item.full_text || '', isImportant: item.is_important }; this.showAnnouncementModal = true },
    closeAnnouncementModal() { this.showAnnouncementModal = false; this.editingAnnouncement = null },
    submitAnnouncement() { if (!this.announcementForm.title || !this.announcementForm.description) { alert('Заполните заголовок и описание'); return } this.editingAnnouncement ? this.updateAnnouncement() : this.createAnnouncement() },
    openGuide() { console.log('Открыть инструкцию') }
  }
}
</script>

<style scoped>
/* ... все существующие стили ... */

/* Добавляем стили для лоадера */
.loading-message {
  text-align: center;
  color: #a2a2a2;
  padding: 40px 20px;
  margin: 0;
  font-size: 14px;
  flex-grow: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
}

.loader {
  width: 30px;
  height: 30px;
  border: 3px solid #f3f3f3;
  border-top: 3px solid #4F5BDF;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.news {
    padding: 20px;
    font-family: 'Montserrat', sans-serif;
    background: linear-gradient(135deg, #f8f9ff 0%, #ffffff 100%);
}

.content-wrapper {
    display: flex;
    gap: 20px;
    align-items: flex-start;
}

.left-column {
    flex: 1;
}

.news-container {
    background: #FFFFFF;
    border: 1px solid #E6E6E6;
    box-shadow: 0px 3px 10px rgba(0, 0, 0, 0.05);
    border-radius: 30px;
    overflow: hidden;
    min-height: 84vh;
}

.news-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0 20px;
    height: 35px;
}

.news-title {
    margin: 0;
    font-weight: 700;
    font-size: 18px;
    line-height: 22px;
    color: #1a1a1a;
}

.header-actions {
    display: flex;
    gap: 8px;
}

.manage-btn {
    width: 115px;
    height: 25px;
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 30px;
    font-family: 'Montserrat', sans-serif;
    font-weight: 500;
    font-size: 13px;
    color: #000000;
    cursor: pointer;
    transition: all 0.2s ease;
}

.manage-btn:hover {
    background: #f5f5f5;
    border-color: #4F5BDF;
}

.divider {
    height: 1px;
    background: #E6E6E6;
    margin: 0;
}

.news-list {
    max-height: 600px;
    overflow-y: scroll;
    padding: 0;
}

.news-item {
    padding: 20px 20px;
    position: relative;
    opacity: 0;
    transform: translateY(20px);
    animation: slideInUp 0.5s ease forwards;
    border-bottom: 1px solid #e6e6e6;
}

@keyframes slideInUp {
    to {
        opacity: 1;
        transform: translateY(0);
    }
}

.news-list::-webkit-scrollbar {
    width: 4px;
}

.news-list::-webkit-scrollbar-track {
    background: transparent;
}

.news-list::-webkit-scrollbar-thumb {
    background: #D9E2FF;
    border-radius: 4px;
}

.news-list::-webkit-scrollbar-thumb:hover {
    background: #4F5BDF;
}

.news-list {
    scrollbar-width: thin;
    scrollbar-color: #D9E2FF transparent;
}

.item-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 6px;
}

.news-date {
    font-weight: 500;
    font-size: 12px;
    line-height: 15px;
    color: #A2A2A2;
}

.item-type {
    font-size: 11px;
    font-weight: 500;
    padding: 2px 8px;
    border-radius: 20px;
    background: #f0f4ff;
    color: #4F5BDF;
}

.news-item-title {
    margin: 0 0 8px 0;
    font-weight: 700;
    font-size: 16px;
    line-height: 20px;
    color: #1a1a1a;
}

.news-item-description {
    margin: 0 0 12px 0;
    font-weight: 400;
    font-size: 14px;
    line-height: 17px;
    color: #666;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
}

.news-details-button {
    height: 25px;
    background: #4F5BDF;
    border: none;
    border-radius: 30px;
    cursor: pointer;
    padding: 0 16px;
    font-family: 'Montserrat', sans-serif;
    font-weight: 500;
    font-size: 12px;
    line-height: 25px;
    color: #FFFFFF;
    transition: background-color 0.2s ease;
}

.news-details-button:hover {
    background: #3a45c5;
}

.empty-state {
    text-align: center;
    padding: 48px 20px;
    color: #a2a2a2;
}

.empty-state p {
    margin: 0;
    font-size: 14px;
}

.right-column {
    width: 450px;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    gap: 10px;
}

.news-card {
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 24px;
    padding: 20px;
    cursor: pointer;
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    opacity: 0;
    transform: translateY(20px);
    animation: slideInUp 0.5s ease forwards;
}

.news-card:hover {
    transform: translateY(-3px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
    border-color: #4F5BDF;
}

.card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
}

.card-type {
    font-size: 11px;
    font-weight: 500;
    padding: 2px 8px;
    border-radius: 20px;
    background: #fff3cd;
    color: #856404;
}

.card-type.important {
    background: #ffb3b3;
    color: #c62828;
}

.card-date {
    font-size: 11px;
    color: #a2a2a2;
}

.card-title {
    margin: 0 0 8px 0;
    font-weight: 700;
    font-size: 16px;
    line-height: 20px;
    color: #1a1a1a;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
}

.card-description {
    margin: 0 0 16px 0;
    font-size: 13px;
    line-height: 1.4;
    color: #666;
    display: -webkit-box;
    -webkit-line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
}

.card-button {
    background: none;
    border: none;
    padding: 0;
    font-family: 'Montserrat', sans-serif;
    font-size: 12px;
    font-weight: 500;
    color: #4F5BDF;
    cursor: pointer;
    transition: transform 0.2s ease;
}

.card-button:hover {
    transform: translateX(4px);
}

.guide-card {
    background: white;
    border: 1px solid #e6e6e6;
    border-radius: 24px;
    padding: 20px;
    width: 450px;
    height: 130px;
    box-sizing: border-box;
    transition: all 0.3s ease;
    cursor: pointer;
    opacity: 0;
    transform: translateY(20px);
    animation: slideInUp 0.5s ease forwards;
}

.guide-card:hover {
    transform: translateY(-3px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
    border: 1px solid #4F5BDF;
}

.guide-content {
    display: flex;
    gap: 16px;
    align-items: flex-start;
}

.guide-icon {
    flex-shrink: 0;
}

.guide-text {
    flex: 1;
}

.guide-title {
    margin: 0 0 12px 0;
    font-weight: 700;
    font-size: 20px;
    line-height: 1.2;
    color: #1a1a1a;
    display: flex;
    gap: 10px;
    align-items: center;
}

.guide-title-blue {
    color: #4F5BDF;
}

.guide-description {
    margin: 0;
    font-size: 12px;
    line-height: 1.4;
    color: #a2a2a2;
    width: 320px;
}

.guide-description strong {
    font-weight: 700;
    color: #a2a2a2;
}

.manage-modal .modal-content {
    width: 680px;
}

.manage-tabs {
    display: flex;
    gap: 8px;
    margin-bottom: 24px;
    border-bottom: 1px solid #e6e6e6;
    padding-bottom: 12px;
}

.tab-btn {
    background: none;
    border: none;
    padding: 8px 20px;
    font-family: 'Montserrat', sans-serif;
    font-size: 14px;
    font-weight: 500;
    color: #a2a2a2;
    cursor: pointer;
    border-radius: 20px;
    transition: all 0.2s ease;
}

.tab-btn:hover {
    color: #4F5BDF;
    background: #f8f9ff;
}

.tab-btn.active {
    color: #4F5BDF;
    background: #f0f4ff;
}

.manage-section {
    margin-top: 8px;
}

.section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
}

.section-title {
    font-size: 14px;
    font-weight: 600;
    color: #333;
}

.add-btn {
    background: #4F5BDF;
    border: none;
    border-radius: 20px;
    padding: 6px 16px;
    font-family: 'Montserrat', sans-serif;
    font-size: 12px;
    font-weight: 500;
    color: white;
    cursor: pointer;
    transition: all 0.2s ease;
}

.add-btn:hover {
    background: #3a45c0;
}

.info-message {
    display: flex;
    align-items: center;
    gap: 8px;
    background: #f0f4ff;
    padding: 10px 14px;
    border-radius: 12px;
    margin-bottom: 16px;
    font-size: 12px;
    color: #4F5BDF;
}

.items-list {
    max-height: 420px;
    overflow-y: auto;
}

.manage-item {
    display: flex;
    justify-content: space-between;
    flex-direction: column;

    padding: 14px;
    border-bottom: 1px solid #f0f0f0;
    transition: background 0.2s ease;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    margin-bottom: 10px;
}

.manage-item:hover {
    background: #fafafa;
}

.item-main {
    flex: 1;
}

.item-title {
    font-weight: 500;
    font-size: 14px;
    color: #333;
    margin-bottom: 6px;
}

.item-meta {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
    font-size: 11px;
    color: #a2a2a2;
}

.status-text {
    color: #666;
}

.active-badge {
    margin-left: 8px;
    padding: 2px 8px;
    background: #4F5BDF;
    color: white;
    border-radius: 12px;
    font-size: 10px;
    font-weight: 500;
}

.important-badge {
    margin-left: 8px;
    padding: 2px 8px;
    background: #ff9800;
    color: white;
    border-radius: 12px;
    font-size: 10px;
    font-weight: 500;
}

.item-actions {
    display: flex;
    gap: 8px;
    padding-top: 10px;
}

.action-btn {
    background: none;
    border: none;
    padding: 3px 12px;
    border-radius: 20px;
    font-family: 'Montserrat', sans-serif;
    font-size: 12px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s ease;
    color: #666;
    border: 1px solid #e6e6e6;
}

.action-btn:hover {
    background: #f0f0f0;
}

.edit-btn:hover {
    color: #4F5BDF;
    background: #f0f4ff;
}

.delete-btn:hover {
    color: #dc2626;
    background: #ffebee;
}

.toggle-btn:hover {
    color: #4F5BDF;
    background: #f0f4ff;
}

.activate-btn:hover {
    color: #4F5BDF;
    background: #f0f4ff;
}

.deactivate-btn:hover {
    color: #dc2626;
    background: #ffebee;
}

.empty-manage {
    text-align: center;
    padding: 48px;
    color: #a2a2a2;
    font-size: 13px;
}

.modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    backdrop-filter: blur(1px);
}

.modal-fade-enter-active,
.modal-fade-leave-active {
    transition: all 0.4s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
    opacity: 0;
}

.modal-fade-enter-from .modal-content,
.modal-fade-leave-to .modal-content {
    opacity: 0;
    transform: scale(0.9) translateY(-20px);
}

.modal-content {
    background: #fff;
    border-radius: 50px;
    width: 540px;
    max-width: 90vw;
    max-height: 80vh;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
    display: flex;
    flex-direction: column;
    overflow: hidden;
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 20px 30px 0;
    flex-shrink: 0;
}

.modal-title {
    margin: 0;
    font-size: 18px;
    font-weight: 600;
    color: #1a1a1a;
}

.modal-close {
    background: none;
    border: none;
    cursor: pointer;
    padding: 6px;
    border-radius: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background-color 0.2s ease;
}

.modal-close:hover {
    background-color: #f5f5f5;
}

.modal-body {
    padding: 20px 30px;
    overflow-y: auto;
    flex: 1;
}

.modal-info {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
}

.modal-date {
    font-size: 12px;
    color: #a2a2a2;
}

.modal-type {
    font-size: 11px;
    font-weight: 500;
    padding: 2px 8px;
    border-radius: 20px;
    background: #f0f4ff;
    color: #4F5BDF;
}

.modal-type.announcement {
    background: #fff3cd;
    color: #856404;
}

.modal-description {
    font-size: 14px;
    line-height: 1.5;
    color: #333;
    margin: 0 0 20px 0;
}

.modal-full-text {
    font-size: 14px;
    line-height: 1.6;
    color: #666;
    padding-top: 16px;
    border-top: 1px solid #f0f0f0;
    margin-top: 8px;
}

.modal-footer {
    padding: 16px 30px 24px;
    border-top: 1px solid #f0f0f0;
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    flex-shrink: 0;
}

.btn {
    padding: 8px 24px;
    font-size: 13px;
    font-weight: 500;
    border-radius: 30px;
    cursor: pointer;
    border: 1px solid;
    transition: all 0.2s ease;
}

.close-btn {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
}

.close-btn:hover {
    background: #3a45c0;
}

.cancel-btn {
    background: white;
    color: #666;
    border-color: #e6e6e6;
}

.cancel-btn:hover {
    background: #f5f5f5;
}

.save-btn {
    background: #4F5BDF;
    color: white;
    border-color: #4F5BDF;
}

.save-btn:hover {
    background: #3a45c0;
}

.form-group {
    margin-bottom: 20px;
}

.form-label {
    display: block;
    font-size: 13px;
    font-weight: 500;
    color: #333;
    margin-bottom: 8px;
}

.form-input,
.form-textarea {
    width: 100%;
    padding: 10px 14px;
    border: 1px solid #e6e6e6;
    border-radius: 20px;
    font-family: 'Montserrat', sans-serif;
    font-size: 13px;
    transition: all 0.2s ease;
    resize: vertical;
}

.form-input:focus,
.form-textarea:focus {
    outline: none;
    border-color: #4F5BDF;
}

.checkbox-label {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    font-size: 13px;
    color: #333;
}

.checkbox-label input {
    width: 16px;
    height: 16px;
    cursor: pointer;
    accent-color: #4F5BDF;
}

@media (max-width: 820px) {
    .content-wrapper {
        flex-direction: column;
    }

    .left-column,
    .right-column {
        width: 100%;
    }

    .guide-card {
        width: 100%;
    }

    .modal-content,
    .manage-modal .modal-content {
        width: 95vw;
    }
}

@media (max-width: 768px) {
    .news-header {
        flex-direction: column;
        align-items: stretch;
        height: auto;
        gap: 10px;
        padding: 12px 14px;
    }

    .news-title {
        font-size: 16px;
    }

    .header-actions {
        width: 100%;
        justify-content: flex-start;
        flex-wrap: wrap;
    }

    .manage-btn {
        flex-shrink: 0;
    }
}
</style>