<template>
  <section class="news">
    <div class="content-wrapper">
      <!-- Левая колонка - Новости -->
      <div class="left-column">
        <div
          class="news-container"
          data-testid="ob-news"
        >
          <div class="news-header">
            <h2 class="news-title">
              Последние новости
            </h2>
            <div class="header-actions">
              <OnboardingButton />
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
          data-testid="ob-guide"
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
                        xlink:href="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAEAAAABACAYAAACqaXHeAAAACXBIWXMAAAHYAAAB2AH6XKZyAAAAGXRFWHRTb2Z0d2FyZQB3d3cuaW5rc2NhcGUub3Jnm+48GgAAAwxJREFUeJzt209oHkUcxvGPMQlqBAk9WCoBJZdoDuLJQyMtlBZSKv4Be6m3QpRcKmoN9RQPpVCCUA+e9FRPCqWlHixeVGwPUpFCrNhi0ItQJH+MQpU2TQ+7L+8SeHl3N7OZTbJfWN7Z3fnNPPMw876zs/PSsL15oGRcP17AM3gknJyg/IwvQxfajynMY3UTHM+HbPwgvqtBo4ocr3drVG/OxvfiC0m3bzGHS1jOWcZaxrA7TV/G94FiDmG0pKaOvKHt6grexYPrLHM6U+Z0wJizKugBxzPpDzCTM6729OTI8zSG0/QiTlcnZ+PJY8BwJn0F/1WkJQp5DHgsk16qSkgs8hiQnSytViUkFnkM2NI0BsQWEJvGgNgCOjCDf2zAhKuOBkzhHTyafr5XZWV1M+AgTq65dgovV1VhnQx4Dp9rP2TdTT978Fl6Pzh1MWAnLmAgPf8dz+K39HxAsrrzROiK62LABIbS9DJexHVJ12+tN+zC0dAV18WA1nR7RfIMP5uez+Kw9nAou4bZkboY0OJtXFxz7ZJkAaYS6mTAp/iow70z+LiKSmMakH20/hpvdsl/LM3XYjGEiLxLYlXwCZ5EH97XHueduIvXJPOC/9P4dRPTgH/xVsGYvzEZUkSdvgOi0BgQW0BsGgNiC4hNY0BsAbFpDIgtIDaNAbEFxKYxILaA2DQGxBYQm8aA2AJis+0NKLskNoZXJRsSd4STE4SniWQuasAQvsGegnGxWOmWoagBm6XhJMvuV7plKjsE/pK8qLiMhZJlVM2cHO8OyhjwE8Zxq0Rs7Sj6K3Abr9gijad4DziHP9L0wziBkaCKwnAPV/Fhml4XR7S3n2f360yq5k8OIY8D3RqXZwhkX2I+lEn/mSM2JityDNU8Gw5G8Eua/hZ7M/d2S3Zu1JEbuBaqsJva3Wo8VKGbiQltA5bwUlw54ci756YXX2Ff5tqPkn+RLeBOYF0h+BXnQxY4KHkOiP3NXuQY69aoIhOhReyX/BTOF4irNWW3nfVJ3B3F4+l5nbiHHwQeAg1bkfsR5B+/y7yHCAAAAABJRU5ErkJggg=="
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
          data-testid="ob-announcement"
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
        <DocumentsBlock />
      </div>
    </div>

    <!-- Модальное окно просмотра новости -->
    <Teleport to="body">
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
                class="modal-full-text news-body-html"
                v-html="sanitizeHtml(selectedNews.full_text)"
              />
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
    </Teleport>

    <!-- Модальное окно просмотра объявления - используем AnnouncementModal -->
    <AnnouncementModal
      :show="showViewAnnouncementModal"
      :announcement="viewingAnnouncement"
      @close="closeViewAnnouncementModal"
    />

    <UserGuideModal
      :show="showGuide"
      :title="guideTitle"
      :sections="guideSections"
      @close="closeGuide"
    />
  </section>
</template>

<script>
import { apiRequest } from '@/api/client'
import RefreshButton from '../components/RefreshButton.vue'
import AnnouncementModal from '../components/AnnouncementModal.vue'
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue'
import UserGuideModal from '../components/news/UserGuideModal.vue'
import { USER_GUIDE_SECTIONS } from '../components/news/userGuideSections.js'
import { ADMIN_GUIDE_SECTIONS } from '../components/news/adminGuideSections.js'
import { usePermissionsStore } from '@/stores/permissions'
import { sanitizeHtml } from '@/utils/sanitize.js'
import DocumentsBlock from '../components/news/DocumentsBlock.vue'
import OnboardingButton from '../components/onboarding/OnboardingButton.vue'

export default {
  name: 'LatestNews',
  components: {
    RefreshButton,
    AnnouncementModal,
    LoaderSpinner,
    UserGuideModal,
    DocumentsBlock,
    OnboardingButton,
  },
  data() {
    return {
      loadingNews: false,
      showNewsDetailsModal: false,
      showViewAnnouncementModal: false,
      showGuide: false,
      selectedNews: null,
      viewingAnnouncement: null,
      newsItems: [],
      activeAnnouncement: null,
    }
  },
  computed: {
    // Руководство админа показываем по праву доступа к админке (page.admin),
    // а не по сырому isSuperAdmin (#187 Фаза 2): admin-режим и явный грант тоже
    // должны видеть admin-руководство.
    isAdminUser() {
      return usePermissionsStore().hasPermission('page.admin');
    },
    guideSections() {
      return this.isAdminUser ? ADMIN_GUIDE_SECTIONS : USER_GUIDE_SECTIONS;
    },
    guideTitle() {
      return this.isAdminUser ? 'Руководство администратора' : 'Руководство пользователя';
    }
  },
  mounted() {
    this.fetchAllData()
  },
  methods: {
    sanitizeHtml,
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
    async fetchAllData() {
      await Promise.all([this.fetchNews(), this.fetchActiveAnnouncement()])
    },

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
    openGuide() { this.showGuide = true; },
    closeGuide() { this.showGuide = false; }
  }
}
</script>

<style scoped>
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
    gap: 20px;
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
    backdrop-filter: blur(0.1px);
    -webkit-backdrop-filter: blur(0.1px);
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

    .modal-content {
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
}

.news-body-html {
    line-height: 1.6;
}
.news-body-html :deep(*) { overflow-wrap: break-word; }
.news-body-html :deep(h1),
.news-body-html :deep(h2),
.news-body-html :deep(h3) { font-weight: 600; margin: 0.75em 0 0.4em; }
.news-body-html :deep(p) { margin: 0.5em 0; }
.news-body-html :deep(ul),
.news-body-html :deep(ol) { padding-left: 1.5em; margin: 0.5em 0; }
.news-body-html :deep(img) { max-width: 100%; border-radius: 8px; }
.news-body-html :deep(img:not([height])) { height: auto; }
.news-body-html :deep(.constructor-image.img-align-left) { float: left; margin: 0 14px 10px 0; }
.news-body-html :deep(.constructor-image.img-align-right) { float: right; margin: 0 0 10px 14px; }
.news-body-html :deep(.constructor-image.img-align-center) { display: block; margin: 10px auto; float: none; }
.news-body-html::after { content: ''; display: block; clear: both; }
.news-body-html :deep(.text-align-left) { text-align: left; }
.news-body-html :deep(.text-align-center) { text-align: center; }
.news-body-html :deep(.text-align-right) { text-align: right; }
</style>
