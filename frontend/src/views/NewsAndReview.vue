<template>
  <section class="news">
    <div class="content-wrapper">
      <!-- Левая колонка - Новости -->
      <div class="left-column">
        <div
          class="news-container"
          data-testid="ob-news"
        >
          <div
            class="news-header"
            data-testid="ob-news-head"
          >
            <h2 class="news-title">
              <span class="news-title__full">Последние новости</span>
              <span class="news-title__short">Новости</span>
            </h2>
            <div class="header-actions">
              <OnboardingMenu />
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
                    viewBox="0 0 24 24"
                    fill="none"
                    xmlns="http://www.w3.org/2000/svg"
                  >
                    <path
                      d="M12 6.5C10.4 5 7.8 4.5 4 4.5V18.5C7.8 18.5 10.4 19 12 20.5C13.6 19 16.2 18.5 20 18.5V4.5C16.2 4.5 13.6 5 12 6.5ZM12 6.5V20.5"
                      stroke="currentColor"
                      stroke-width="1.6"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                    />
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

        <button
          type="button"
          class="modes-trigger"
          data-testid="ob-work-modes"
          @click="openModes"
        >
          <span class="modes-trigger__icon">
            <svg
              width="22"
              height="22"
              viewBox="0 0 24 24"
              fill="none"
              xmlns="http://www.w3.org/2000/svg"
            >
              <circle
                cx="12"
                cy="12"
                r="9"
                stroke="currentColor"
                stroke-width="1.7"
              />
              <path
                d="M12 7v5l3.5 2"
                stroke="currentColor"
                stroke-width="1.7"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
          </span>
          <span class="modes-trigger__text">
            <span class="modes-trigger__title">Режимы работы</span>
            <span class="modes-trigger__sub">Время работы Бюро, мест разгрузки и мест прохода</span>
          </span>
          <svg
            class="modes-trigger__chev"
            width="16"
            height="16"
            viewBox="0 0 16 16"
            fill="none"
          >
            <path
              d="M6 4l4 4-4 4"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </svg>
        </button>
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
          <div
            class="modal-content"
            :class="{ 'is-dragging': sheetDragging }"
            :style="sheetOffset ? { transform: `translateY(${sheetOffset}px)` } : null"
            @touchstart="onSheetTouchStart"
            @touchmove="onSheetTouchMove"
            @touchend="onSheetTouchEnd"
          >
            <!-- Ползунок bottom-sheet (виден только на мобилке), свайп вниз закрывает -->
            <div
              class="sheet-handle"
              aria-hidden="true"
            />
            <div
              ref="sheetBody"
              class="modal-body"
            >
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
      :sections="guideSections"
      :loading="guideLoading"
      @close="closeGuide"
    />

    <WorkModesModal
      :show="showModes"
      @close="closeModes"
    />
  </section>
</template>

<script>
import { apiRequest } from '@/api/client'
import eventStream from '@/services/eventStream'
import RefreshButton from '../components/RefreshButton.vue'
import AnnouncementModal from '../components/AnnouncementModal.vue'
import LoaderSpinner from '@/components/ui/LoaderSpinner.vue'
import UserGuideModal from '../components/news/UserGuideModal.vue'
import { listGuideSections } from '@/api/guide'
import { useDeletionsStore } from '@/stores/deletions'
import { sanitizeHtml } from '@/utils/sanitize.js'
import DocumentsBlock from '../components/news/DocumentsBlock.vue'
import WorkModesModal from '../components/news/WorkModesModal.vue'
import OnboardingMenu from '../components/onboarding/OnboardingMenu.vue'
import { ref } from 'vue'
import { useSwipeDismiss } from '@/composables/useSwipeDismiss'
import { useOnboardingStore } from '@/stores/onboarding'
import { openItemFromRoute } from '@/utils/openQueryParam'

export default {
  name: 'LatestNews',
  components: {
    RefreshButton,
    AnnouncementModal,
    LoaderSpinner,
    UserGuideModal,
    DocumentsBlock,
    WorkModesModal,
    OnboardingMenu,
  },
  setup() {
    // Читалка новости на мобилке - bottom-sheet со свайп-вниз-закрытием (W3.4).
    // Состояние держим в setup, чтобы onDismiss свайпа мог закрыть модалку.
    const showNewsDetailsModal = ref(false)
    const selectedNews = ref(null)
    const sheetBody = ref(null)
    const openNewsModal = (item) => {
      selectedNews.value = item
      showNewsDetailsModal.value = true
    }
    const closeNewsDetailsModal = () => {
      showNewsDetailsModal.value = false
      selectedNews.value = null
    }
    const swipe = useSwipeDismiss(closeNewsDetailsModal, {
      getScrollTop: () => sheetBody.value?.scrollTop ?? 0,
      handleSelector: '.sheet-handle',
    })
    return {
      showNewsDetailsModal,
      selectedNews,
      sheetBody,
      openNewsModal,
      closeNewsDetailsModal,
      sheetOffset: swipe.offset,
      sheetDragging: swipe.isDragging,
      onSheetTouchStart: swipe.onTouchStart,
      onSheetTouchMove: swipe.onTouchMove,
      onSheetTouchEnd: swipe.onTouchEnd,
      onboardingStore: useOnboardingStore(),
    }
  },
  data() {
    return {
      loadingNews: false,
      showViewAnnouncementModal: false,
      showGuide: false,
      showModes: false,
      // Расписание открыл тур - только такое окно он и закрывает за собой.
      modesOpenedByTour: false,
      viewingAnnouncement: null,
      newsItems: [],
      activeAnnouncement: null,
      guideSections: [],
      guideLoading: false,
      guideLoaded: false,
      eventStreamOff: null,
    }
  },
  mounted() {
    this.fetchAllData()
    window.addEventListener('keydown', this.handleEscKey)

    // Real-time доставка (#840 news.refresh): по сигналу сервера мгновенно
    // перезапрашиваем новости и активное объявление вместо ожидания F5.
    eventStream.connect()
    this.eventStreamOff = eventStream.subscribe('news', () => {
      this.fetchAllData()
    })
  },
  beforeUnmount() {
    window.removeEventListener('keydown', this.handleEscKey)
    if (this.eventStreamOff) {
      this.eventStreamOff()
      this.eventStreamOff = null
    }
    eventStream.disconnect()
  },
  watch: {
    // Пользователь уже на этой странице: повторного монтирования нет, а адрес сменился.
    '$route.query.open'(val) { if (val) this.openFromSearchLink(); },
    /**
     * Онбординг просит показать расписание: открываем окно режимов работы по
     * сигналу и закрываем, когда сигнал гаснет. Чужое окно не трогаем.
     */
    'onboardingStore.revealOpen'(target) {
      if (target === 'work-modes') {
        if (this.showModes) return
        this.modesOpenedByTour = true
        this.showModes = true
        return
      }
      if (!this.modesOpenedByTour) return
      this.modesOpenedByTour = false
      this.showModes = false
    },
  },
  methods: {
    /** Переход из сквозного поиска: `?open` раскрывает найденную новость. */
    openFromSearchLink() {
      openItemFromRoute({ router: this.$router, route: this.$route, items: this.newsItems, open: this.openNewsModal })
    },

    sanitizeHtml,
    handleEscKey(e) {
      if (e.key === 'Escape' && this.showNewsDetailsModal) this.closeNewsDetailsModal()
    },
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
        this.openFromSearchLink()
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
    // Разделы руководства гейтятся правами на бэке (GET /guide/sections отдаёт
    // только доступные роли), фронт рисует ровно пришедшее. Грузим лениво на
    // первом открытии модалки.
    async loadGuideSections() {
      this.guideLoading = true
      try {
        const data = await listGuideSections()
        this.guideSections = Array.isArray(data) ? data : []
        this.guideLoaded = true
      } catch {
        useDeletionsStore().notify({ prefix: 'Ошибка загрузки руководства', type: 'error' })
      } finally {
        this.guideLoading = false
      }
    },

    openAnnouncementModal(announcement) {
      this.viewingAnnouncement = announcement;
      this.showViewAnnouncementModal = true;
    },
    closeViewAnnouncementModal() {
      this.showViewAnnouncementModal = false;
      this.viewingAnnouncement = null;
    },
    openGuide() {
      this.showGuide = true;
      // guideLoading в условии — защита от параллельной загрузки при быстром
      // открыл/закрыл/открыл, пока первый запрос ещё в полёте.
      if (!this.guideLoaded && !this.guideLoading) this.loadGuideSections();
    },
    closeGuide() { this.showGuide = false; },
    openModes() { this.showModes = true; },
    closeModes() { this.showModes = false; }
  }
}
</script>

<style scoped>
/* Добавляем стили для лоадера */
.loading-message {
  text-align: center;
  color: var(--text-muted);
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
  border: 3px solid var(--surface-2);
  border-top: 3px solid var(--accent);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

/* Фон не задаём: это страница, её фон - --bg от body. Своя заливка в --surface делала
   «Обзор и новости» единственным экраном с карточным цветом во всю площадь. */
.news {
    padding: 20px;
    font-family: 'Montserrat', sans-serif;
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
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 30px;
    overflow: hidden;
    /* var(--app-vh) = зумленная высота вьюпорта (viewportScale.js); чистый vh
       под корневым zoom считается от незумленной высоты и завышает карточку. */
    min-height: calc(var(--app-vh, 1vh) * 84);
    /* B.3 (#1097): svh стабилизирует высоту на мобилке (ретракт адрес-бара браузера не
       дёргает layout); min() держит zoom-корректность на десктопе; фолбэк на calc выше. */
    min-height: min(calc(var(--app-vh, 1vh) * 84), 84svh);
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
    color: var(--text);
}

.header-actions {
    display: flex;
    gap: 8px;
}

.divider {
    height: 1px;
    background: var(--border);
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
    border-bottom: 1px solid var(--border);
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
    background: color-mix(in srgb, var(--accent) 22%, var(--surface));
    border-radius: 4px;
}

.news-list::-webkit-scrollbar-thumb:hover {
    background: var(--accent);
}

.news-list {
    scrollbar-width: thin;
    scrollbar-color: color-mix(in srgb, var(--accent) 22%, var(--surface)) transparent;
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
    color: var(--text-muted);
}

.item-type {
    font-size: 11px;
    font-weight: 500;
    padding: 2px 8px;
    border-radius: 20px;
    background: var(--accent-tint);
    color: var(--accent-text);
}

.news-item-title {
    margin: 0 0 8px 0;
    font-weight: 700;
    font-size: 16px;
    line-height: 20px;
    color: var(--text);
}

.news-item-description {
    margin: 0 0 12px 0;
    font-weight: 400;
    font-size: 14px;
    line-height: 17px;
    color: var(--text-muted);
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
}

.news-details-button {
    height: 25px;
    background: var(--accent);
    border: none;
    border-radius: 30px;
    cursor: pointer;
    padding: 0 16px;
    font-family: 'Montserrat', sans-serif;
    font-weight: 500;
    font-size: 12px;
    line-height: 25px;
    color: var(--accent-contrast);
    transition: background-color 0.2s ease;
}

.news-details-button:hover {
    background: var(--accent-hover);
}

.empty-state {
    text-align: center;
    padding: 48px 20px;
    color: var(--text-muted);
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
    background: var(--surface);
    border: 1px solid var(--border);
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
    box-shadow: 0 4px 12px var(--shadow-drop);
    border-color: var(--accent);
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
    background: var(--warning-bg);
    border: 1px solid color-mix(in srgb, var(--warning) 42%, var(--surface));
    color: var(--warning-text);
}

.card-type.important {
    background: var(--danger-bg);
    color: var(--danger-text);
}

.card-date {
    font-size: 11px;
    color: var(--text-muted);
}

.card-title {
    margin: 0 0 8px 0;
    font-weight: 700;
    font-size: 16px;
    line-height: 20px;
    color: var(--text);
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
}

.card-description {
    margin: 0 0 16px 0;
    font-size: 13px;
    line-height: 1.4;
    color: var(--text-muted);
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
    color: var(--accent-text);
    cursor: pointer;
    transition: transform 0.2s ease;
}

.card-button:hover {
    transform: translateX(4px);
}

.guide-card {
    background: var(--surface);
    border: 1px solid var(--border);
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
    box-shadow: 0 4px 12px var(--shadow-drop);
    border: 1px solid var(--accent);
}

.modes-trigger {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 16px 18px;
    background: var(--surface);
    border: 1px solid var(--color-border);
    border-radius: 22px;
    font-family: 'Montserrat', sans-serif;
    text-align: left;
    cursor: pointer;
    transition: transform 0.2s ease, border-color 0.2s ease;
}

.modes-trigger:hover {
    border-color: var(--accent);
}

.modes-trigger__icon {
    width: 42px;
    height: 42px;
    flex-shrink: 0;
    border-radius: 12px;
    background: var(--accent-tint);
    color: var(--accent-text);
    display: inline-flex;
    align-items: center;
    justify-content: center;
}

.modes-trigger__text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 3px;
}

.modes-trigger__title {
    font-size: 15px;
    font-weight: 700;
    color: var(--text);
}

.modes-trigger__sub {
    font-size: 12px;
    color: var(--text-muted);
    font-weight: 500;
}

.modes-trigger__chev {
    flex-shrink: 0;
    color: var(--text-muted);
    transition: transform 0.2s ease, color 0.2s ease;
}

.modes-trigger:hover .modes-trigger__chev {
    color: var(--accent-text);
    transform: translateX(3px);
}

.guide-content {
    display: flex;
    gap: 16px;
    align-items: flex-start;
}

.guide-icon {
    flex-shrink: 0;
    /* Иконка нарисована currentColor - цвет берётся отсюда и следует за темой. */
    color: var(--accent-text);
}

.guide-text {
    flex: 1;
}

.guide-title {
    margin: 0 0 12px 0;
    font-weight: 700;
    font-size: 20px;
    line-height: 1.2;
    color: var(--text);
    display: flex;
    gap: 10px;
    align-items: center;
}

.guide-title-blue {
    color: var(--accent-text);
}

.guide-description {
    margin: 0;
    font-size: 12px;
    line-height: 1.4;
    color: var(--text-muted);
    width: 320px;
}

.guide-description strong {
    font-weight: 700;
    color: var(--text-muted);
}

.modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: var(--overlay);
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
    background: var(--surface);
    border-radius: 50px;
    width: 540px;
    max-width: 90vw;
    max-height: calc(var(--app-vh, 1vh) * 80);
    box-shadow: 0 20px 60px var(--shadow-drop);
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
    padding-bottom: 8px;
    font-size: 18px;
    font-weight: 600;
    color: var(--text);
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
    color: var(--text-muted);
}

.modal-type {
    font-size: 11px;
    font-weight: 500;
    padding: 2px 8px;
    border-radius: 20px;
    background: var(--accent-tint);
    color: var(--accent-text);
}

.modal-description {
    font-size: 14px;
    line-height: 1.5;
    color: var(--text);
    margin: 0 0 20px 0;
}

.modal-full-text {
    font-size: 14px;
    line-height: 1.6;
    color: var(--text-muted);
    padding-top: 16px;
    border-top: 1px solid var(--border);
    margin-top: 8px;
}

.modal-footer {
    padding: 16px 30px 24px;
    border-top: 1px solid var(--border);
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
    background: var(--accent);
    color: var(--accent-contrast);
    border-color: var(--accent);
}

.close-btn:hover {
    background: var(--accent-hover);
}

/* Ползунок bottom-sheet и короткий заголовок - только на мобилке (@media768). */
.sheet-handle {
    display: none;
}

.news-title__short {
    display: none;
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
    /* Боковой отступ страницы на телефоне ужимаем. */
    .news {
        padding: 15px;
    }

    /* Заголовок в строку с кнопками: короткий текст «Новости» помещается
       рядом с «Обучение» и «Обновить». На очень узких (<=320) кнопки уходят
       на вторую строку (flex-wrap) - лучше, чем горизонтальный скролл страницы. */
    .news-header {
        flex-direction: row;
        align-items: center;
        flex-wrap: wrap;
        height: auto;
        gap: 8px;
        padding: 12px 14px;
    }

    .news-title {
        font-size: 16px;
        white-space: nowrap;
    }

    .news-title__full {
        display: none;
    }

    .news-title__short {
        display: inline;
    }

    .header-actions {
        width: auto;
        margin-left: auto;
        justify-content: flex-end;
        flex-wrap: nowrap;
        gap: 6px;
    }

    /* "Обновить" на мобилке - только иконка; высота как у кнопки "Обучение" (25px).
       Ширина ФИКСИРОВАНА под самое широкое состояние (3 точки перезарядки = 45px),
       иначе кнопка дёргается на 5px при переходе иконка<->точки (обзор 1 R3-7). */
    .header-actions :deep(.refresh-btn) {
        height: 25px;
        width: 45px;
        box-sizing: border-box;
        padding: 0 8px;
        justify-content: center;
        border-radius: 50px;
    }
    .header-actions :deep(.refresh-btn__text) {
        display: none;
    }

    /* Блок "Руководство" (обзор 2): фикс width:450px/height:130px и description
       width:320px ломали карточку на узком экране (текст вылезал за блок). Высота
       по контенту, описание на всю ширину, иконку в строке заголовка убираем -
       заголовок больше не "кривой". */
    .guide-card {
        height: auto;
    }
    .guide-icon {
        display: none;
    }
    .guide-description {
        width: 100%;
    }

    /* Читалка новости - bottom-sheet: оверлей прижимает лист к низу,
       лист во всю ширину выезжает снизу; свайп вниз за ползунок закрывает. */
    .modal-overlay {
        align-items: flex-end;
    }

    .modal-content {
        width: 100%;
        max-width: 100%;
        max-height: 88vh;
        border-radius: 16px 16px 0 0;
        transition: transform 0.3s ease;
    }

    /* Пока тянем пальцем - без анимации (лист следует за пальцем 1:1). */
    .modal-content.is-dragging {
        transition: none;
    }

    .sheet-handle {
        display: block;
        width: 40px;
        height: 4px;
        margin: 10px auto 2px;
        border-radius: 2px;
        background: var(--border);
        flex-shrink: 0;
    }

    /* Вход/выход - выезд снизу вместо десктопного scale (переопределяем .modal-fade). */
    .modal-fade-enter-from .modal-content,
    .modal-fade-leave-to .modal-content {
        opacity: 1;
        transform: translateY(100%);
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
