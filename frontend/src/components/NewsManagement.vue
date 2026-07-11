<template>
  <div class="news-management">
    <!-- Шапка -->
    <div class="management-header rt-header-inline">
      <h2 class="management-title">
        Новости и объявления
      </h2>
      <div class="management-header__actions header-controls">
        <div class="tab-group">
          <button
            class="tab-btn"
            :class="{ active: activeTab === 'news' }"
            @click="switchTab('news')"
          >
            Новости
          </button>
          <button
            class="tab-btn"
            :class="{ active: activeTab === 'announcements' }"
            @click="switchTab('announcements')"
          >
            Объявления
          </button>
        </div>
        <button
          class="lk-button lk-button--primary add-btn rt-btn-compact"
          :aria-label="activeTab === 'news' ? 'Добавить новость' : 'Добавить объявление'"
          @click="openCreateModal"
        >
          <span
            class="rt-btn-icon"
            aria-hidden="true"
          >+</span>
          <!-- "+" остаётся частью rt-btn-label (не только rt-btn-icon) - на
               десктопе rt-btn-icon скрыт по умолчанию (см. responsive-tables.css),
               исходная кнопка всегда показывала "+ Добавить..." - не убираем. -->
          <span class="rt-btn-label">+ {{ activeTab === 'news' ? 'Добавить новость' : 'Добавить объявление' }}</span>
        </button>
        <RefreshButton
          :loading="loading"
          @refresh="fetchAll"
        />
      </div>
    </div>

    <!-- Контент -->
    <div class="management-body">
      <transition
        name="tab-fade"
        mode="out-in"
      >
        <!-- Список новостей -->
        <div
          v-if="activeTab === 'news'"
          key="news"
          class="items-list"
        >
          <div
            v-if="loading"
            class="empty-state"
          >
            Загрузка…
          </div>
          <div
            v-else-if="newsItems.length === 0"
            class="empty-state"
          >
            Нет новостей
          </div>
          <div
            v-for="item in newsItems"
            v-else
            :key="item.id"
            class="manage-item"
            :class="{ selected: selectedItem?.id === item.id }"
            @click="selectItem(item)"
          >
            <div class="item-main">
              <div class="item-title">
                {{ item.title }}
                <span
                  v-if="!item.is_active"
                  class="badge badge--archive"
                >(скрыта)</span>
              </div>
              <div class="item-meta">
                <span>{{ formatDate(item.created_at) }}</span>
                <span>{{ item.created_by_name || 'Система' }}</span>
                <span
                  class="status-chip"
                  :class="item.is_active ? 'status-chip--active' : 'status-chip--hidden'"
                >{{ item.is_active ? 'Активна' : 'Скрыта' }}</span>
              </div>
            </div>
          </div>
          <div class="items-footer">
            Всего: {{ newsItems.length }}
          </div>
        </div>

        <!-- Список объявлений -->
        <div
          v-else-if="activeTab === 'announcements'"
          key="announcements"
          class="items-list"
        >
          <div class="info-note">
            Активно может быть только одно объявление
          </div>
          <div
            v-if="loading"
            class="empty-state"
          >
            Загрузка…
          </div>
          <div
            v-else-if="announcementsItems.length === 0"
            class="empty-state"
          >
            Нет объявлений
          </div>
          <div
            v-for="item in announcementsItems"
            v-else
            :key="item.id"
            class="manage-item"
            :class="{ selected: selectedItem?.id === item.id }"
            @click="selectItem(item)"
          >
            <div class="item-main">
              <div class="item-title">
                {{ item.title }}
                <span
                  v-if="item.is_important"
                  class="badge badge--important"
                >Важное</span>
                <span
                  v-if="item.is_active"
                  class="badge badge--active"
                >Активно</span>
              </div>
              <div class="item-meta">
                <span>{{ formatDate(item.created_at) }}</span>
                <span>{{ item.created_by_name || 'Система' }}</span>
                <span v-if="item.activated_by_name">Активировал: {{ item.activated_by_name }}</span>
              </div>
            </div>
          </div>
          <div class="items-footer">
            Всего: {{ announcementsItems.length }}
          </div>
        </div>
      </transition>

      <!-- Панель деталей -->
      <div
        v-if="selectedItem"
        class="detail-panel"
      >
        <div class="detail-header">
          <h3 class="detail-title">
            {{ selectedItem.title }}
          </h3>
          <div class="detail-actions">
            <button
              class="lk-button lk-button--secondary"
              @click="openEditModal"
            >
              Редактировать
            </button>
            <template v-if="activeTab === 'news'">
              <button
                class="lk-button lk-button--secondary"
                @click="toggleNewsActive(selectedItem)"
              >
                {{ selectedItem.is_active ? 'Скрыть' : 'Показать' }}
              </button>
            </template>
            <template v-else>
              <button
                v-if="!selectedItem.is_active"
                class="lk-button lk-button--secondary"
                @click="setActiveAnnouncement(selectedItem.id)"
              >
                Показать
              </button>
              <button
                v-if="selectedItem.is_active"
                class="lk-button lk-button--secondary"
                @click="deactivateAnnouncement(selectedItem.id)"
              >
                Скрыть
              </button>
            </template>
            <button
              class="lk-button lk-button--danger"
              @click="deleteItem(selectedItem)"
            >
              Удалить
            </button>
          </div>
        </div>
        <div class="detail-body">
          <div class="detail-meta">
            <span>{{ formatDate(selectedItem.created_at) }}</span>
            <span v-if="selectedItem.created_by_name">{{ selectedItem.created_by_name }}</span>
          </div>
          <p class="detail-description">
            {{ selectedItem.description }}
          </p>
          <div
            v-if="selectedItem.full_text"
            class="detail-full-text news-body-html"
            v-html="sanitizeHtml(selectedItem.full_text)"
          />
        </div>
      </div>
      <div
        v-else
        class="detail-panel detail-panel--empty"
      >
        <span>Выберите элемент из списка</span>
      </div>
    </div>

    <!-- Модалка создания/редактирования новости -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="showNewsModal"
          class="modal-overlay"
          @click.self="closeNewsModal"
        >
          <div class="modal-content">
            <div class="modal-header">
              <h3 class="modal-title">
                {{ editingItem ? 'Редактировать новость' : 'Создать новость' }}
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
                <label class="form-label">Заголовок</label>
                <input
                  v-model="newsForm.title"
                  type="text"
                  class="lk-input"
                  placeholder="Введите заголовок"
                >
              </div>
              <div class="form-group">
                <label class="form-label">Краткое описание</label>
                <textarea
                  v-model="newsForm.description"
                  class="lk-textarea"
                  placeholder="Введите краткое описание"
                  rows="3"
                />
              </div>
              <div class="form-group">
                <label class="form-label">Полный текст</label>
                <TextConstructor v-model="newsForm.fullText" />
              </div>
            </div>
            <div class="modal-footer">
              <button
                class="lk-button lk-button--secondary"
                @click="closeNewsModal"
              >
                Отмена
              </button>
              <button
                class="lk-button lk-button--primary"
                @click="submitNews"
              >
                Сохранить
              </button>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>

    <!-- Модалка создания/редактирования объявления -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="showAnnouncementModal"
          class="modal-overlay"
          @click.self="closeAnnouncementModal"
        >
          <div class="modal-content">
            <div class="modal-header">
              <h3 class="modal-title">
                {{ editingItem ? 'Редактировать объявление' : 'Создать объявление' }}
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
                <label class="form-label">Заголовок</label>
                <input
                  v-model="announcementForm.title"
                  type="text"
                  class="lk-input"
                  placeholder="Введите заголовок"
                >
              </div>
              <div class="form-group">
                <label class="form-label">Краткое описание</label>
                <textarea
                  v-model="announcementForm.description"
                  class="lk-textarea"
                  placeholder="Введите краткое описание"
                  rows="3"
                />
              </div>
              <div class="form-group">
                <label class="form-label">Полный текст</label>
                <TextConstructor v-model="announcementForm.fullText" />
              </div>
              <div class="form-group">
                <label class="checkbox-label">
                  <input
                    v-model="announcementForm.isImportant"
                    type="checkbox"
                  >
                  <span>Важное объявление</span>
                </label>
              </div>
            </div>
            <div class="modal-footer">
              <button
                class="lk-button lk-button--secondary"
                @click="closeAnnouncementModal"
              >
                Отмена
              </button>
              <button
                class="lk-button lk-button--primary"
                @click="submitAnnouncement"
              >
                Сохранить
              </button>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client';
import RefreshButton from '@/components/RefreshButton.vue';
import TextConstructor from '@/components/TextConstructor.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { useUiStore } from '@/stores/ui';
import { sanitizeHtml } from '@/utils/sanitize.js';

export default {
  name: 'NewsManagement',
  components: { RefreshButton, TextConstructor },
  data() {
    return {
      loading: false,
      activeTab: 'news',
      selectedItem: null,
      newsItems: [],
      announcementsItems: [],
      showNewsModal: false,
      showAnnouncementModal: false,
      editingItem: null,
      newsForm: { title: '', description: '', fullText: '' },
      announcementForm: { title: '', description: '', fullText: '', isImportant: false },
    };
  },
  mounted() {
    this.fetchAll();
  },
  methods: {
    sanitizeHtml,
    formatDate(dateString) {
      if (!dateString) return '';
      const date = new Date(dateString);
      const mskDate = new Date(date.getTime() + 3 * 60 * 60 * 1000);
      return mskDate.toLocaleDateString('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      }).replace(',', '');
    },

    switchTab(tab) {
      this.activeTab = tab;
      this.selectedItem = null;
    },

    async fetchAll() {
      this.loading = true;
      try {
        await Promise.all([this.fetchNews(), this.fetchAnnouncements()]);
      } finally {
        this.loading = false;
      }
    },
    async fetchNews() {
      try {
        const res = await apiRequest('/news/all');
        if (res.ok) this.newsItems = await res.json();
      } catch (e) {
        console.error('Ошибка загрузки новостей:', e);
      }
    },
    async fetchAnnouncements() {
      try {
        const res = await apiRequest('/announcements/all');
        if (res.ok) this.announcementsItems = await res.json();
      } catch (e) {
        console.error('Ошибка загрузки объявлений:', e);
      }
    },

    selectItem(item) {
      this.selectedItem = item;
    },

    // --- Новости ---
    openCreateModal() {
      if (this.activeTab === 'news') {
        this.editingItem = null;
        this.newsForm = { title: '', description: '', fullText: '' };
        this.showNewsModal = true;
      } else {
        this.editingItem = null;
        this.announcementForm = { title: '', description: '', fullText: '', isImportant: false };
        this.showAnnouncementModal = true;
      }
    },
    openEditModal() {
      if (this.activeTab === 'news') {
        this.editingItem = this.selectedItem;
        this.newsForm = {
          title: this.selectedItem.title,
          description: this.selectedItem.description,
          fullText: this.selectedItem.full_text || '',
        };
        this.showNewsModal = true;
      } else {
        this.editingItem = this.selectedItem;
        this.announcementForm = {
          title: this.selectedItem.title,
          description: this.selectedItem.description,
          fullText: this.selectedItem.full_text || '',
          isImportant: this.selectedItem.is_important,
        };
        this.showAnnouncementModal = true;
      }
    },
    closeNewsModal() {
      this.showNewsModal = false;
      this.editingItem = null;
    },
    closeAnnouncementModal() {
      this.showAnnouncementModal = false;
      this.editingItem = null;
    },
    async submitNews() {
      if (!this.newsForm.title || !this.newsForm.description) {
        useDeletionsStore().notify({ bold: 'Заполните заголовок и описание', type: 'error' });
        return;
      }
      const payload = {
        title: this.newsForm.title,
        description: this.newsForm.description,
        full_text: this.newsForm.fullText,
      };
      try {
        let res;
        if (this.editingItem) {
          res = await apiRequest(`/news/${this.editingItem.id}`, {
            method: 'PUT',
            body: JSON.stringify(payload),
          });
        } else {
          res = await apiRequest('/news', { method: 'POST', body: JSON.stringify(payload) });
        }
        if (res.ok) {
          this.closeNewsModal();
          await this.fetchNews();
          useDeletionsStore().notify({
            prefix: this.editingItem ? 'Новость обновлена' : 'Новость создана',
          });
        } else {
          useDeletionsStore().notify({
            bold: this.editingItem ? 'Ошибка обновления новости' : 'Ошибка создания новости',
            type: 'error',
          });
        }
      } catch (e) {
        console.error('Ошибка сохранения новости:', e);
      }
    },
    async toggleNewsActive(item) {
      try {
        const res = await apiRequest(`/news/${item.id}`, {
          method: 'PUT',
          body: JSON.stringify({ is_active: !item.is_active }),
        });
        if (res.ok) {
          await this.fetchNews();
          this.selectedItem = this.newsItems.find((n) => n.id === item.id) || null;
          useDeletionsStore().notify({
            prefix: item.is_active ? 'Новость скрыта' : 'Новость показана',
          });
        } else {
          useDeletionsStore().notify({ bold: 'Ошибка изменения статуса', type: 'error' });
        }
      } catch (e) {
        console.error('Ошибка изменения статуса новости:', e);
      }
    },

    // --- Объявления ---
    async submitAnnouncement() {
      if (!this.announcementForm.title || !this.announcementForm.description) {
        useDeletionsStore().notify({ bold: 'Заполните заголовок и описание', type: 'error' });
        return;
      }
      const payload = {
        title: this.announcementForm.title,
        description: this.announcementForm.description,
        full_text: this.announcementForm.fullText,
        is_important: this.announcementForm.isImportant,
      };
      try {
        let res;
        if (this.editingItem) {
          res = await apiRequest(`/announcements/${this.editingItem.id}`, {
            method: 'PUT',
            body: JSON.stringify(payload),
          });
        } else {
          res = await apiRequest('/announcements', { method: 'POST', body: JSON.stringify(payload) });
        }
        if (res.ok) {
          this.closeAnnouncementModal();
          await this.fetchAnnouncements();
          useDeletionsStore().notify({
            prefix: this.editingItem ? 'Объявление обновлено' : 'Объявление создано',
          });
        } else {
          useDeletionsStore().notify({
            bold: this.editingItem ? 'Ошибка обновления объявления' : 'Ошибка создания объявления',
            type: 'error',
          });
        }
      } catch (e) {
        console.error('Ошибка сохранения объявления:', e);
      }
    },
    async setActiveAnnouncement(id) {
      const ok = await useUiStore().confirm({
        title: 'Показать объявление',
        message: 'Активировать это объявление? Текущее активное будет скрыто.',
        confirmText: 'Показать',
        danger: false,
      });
      if (!ok) return;
      try {
        const res = await apiRequest('/announcements/set-active', {
          method: 'POST',
          body: JSON.stringify({ announcement_id: id }),
        });
        if (res.ok) {
          await this.fetchAnnouncements();
          this.selectedItem = this.announcementsItems.find((a) => a.id === id) || null;
          useDeletionsStore().notify({ prefix: 'Объявление активировано' });
        } else {
          useDeletionsStore().notify({ bold: 'Ошибка активации объявления', type: 'error' });
        }
      } catch (e) {
        console.error('Ошибка активации объявления:', e);
      }
    },
    async deactivateAnnouncement(id) {
      const ok = await useUiStore().confirm({
        title: 'Скрыть объявление',
        message: 'Скрыть активное объявление?',
        confirmText: 'Скрыть',
        danger: false,
      });
      if (!ok) return;
      try {
        const res = await apiRequest(`/announcements/${id}/hide`, { method: 'POST' });
        if (res.ok) {
          await this.fetchAnnouncements();
          this.selectedItem = this.announcementsItems.find((a) => a.id === id) || null;
          useDeletionsStore().notify({ prefix: 'Объявление скрыто' });
        } else {
          useDeletionsStore().notify({ bold: 'Ошибка скрытия объявления', type: 'error' });
        }
      } catch (e) {
        console.error('Ошибка скрытия объявления:', e);
      }
    },

    async deleteItem(item) {
      const label = this.activeTab === 'news' ? 'новость' : 'объявление';
      const ok = await useUiStore().confirm({
        title: `Удалить ${label}`,
        message: `Удалить "${item.title}"? Это действие необратимо.`,
        confirmText: 'Удалить',
        danger: true,
      });
      if (!ok) return;

      const endpoint = this.activeTab === 'news'
        ? `/news/${item.id}`
        : `/announcements/${item.id}`;

      const deletedTitle = item.title;
      try {
        const res = await apiRequest(endpoint, { method: 'DELETE' });
        if (res.ok) {
          if (this.activeTab === 'news') {
            await this.fetchNews();
          } else {
            await this.fetchAnnouncements();
          }
          this.selectedItem = null;
          useDeletionsStore().notify({
            prefix: this.activeTab === 'news' ? 'Новость удалена: ' : 'Объявление удалено: ',
            bold: deletedTitle,
          });
        } else {
          useDeletionsStore().notify({ bold: `Ошибка удаления: ${deletedTitle}`, type: 'error' });
        }
      } catch (e) {
        console.error('Ошибка удаления:', e);
      }
    },
  },
};
</script>

<style scoped>
.news-management {
  display: flex;
  flex-direction: column;
  height: 100%;
  font-family: 'Montserrat', sans-serif;
  background: #fff;
  border: 1px solid #e6e6e6;
  border-radius: 35px;
  overflow: hidden;
}

/* Шапка */
.management-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 24px;
  height: 50px;
  border-bottom: 1px solid #e6e6e6;
  flex-shrink: 0;
}

.management-title {
  margin: 0;
  font-size: 1.2em;
  font-weight: 600;
  color: #000;
}

.management-header__actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

/* Табы */
.tab-group {
  display: flex;
  gap: 4px;
  background: #f5f5f5;
  border-radius: 20px;
  padding: 3px;
}

.tab-btn {
  background: none;
  border: none;
  padding: 5px 16px;
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  font-weight: 500;
  color: #a2a2a2;
  cursor: pointer;
  border-radius: 17px;
  transition: all 0.2s ease;
}

.tab-btn:hover {
  color: #4F5BDF;
}

.tab-btn.active {
  background: #fff;
  color: #4F5BDF;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

/* Тело: список + детали */
.management-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

/* Левый список */
.items-list {
  width: 360px;
  flex-shrink: 0;
  border-right: 1px solid #e6e6e6;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}

.items-list::-webkit-scrollbar {
  width: 4px;
}

.items-list::-webkit-scrollbar-thumb {
  background: #D9E2FF;
  border-radius: 4px;
}

.empty-state {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
  color: #a2a2a2;
  font-size: 13px;
}

.info-note {
  margin: 10px 12px 0;
  padding: 8px 12px;
  background: #f0f4ff;
  border-radius: 10px;
  font-size: 12px;
  color: #4F5BDF;
}

.manage-item {
  padding: 14px 16px;
  border-bottom: 1px solid #f0f0f0;
  cursor: pointer;
  transition: background 0.15s ease;
}

.manage-item:hover {
  background: #fafafa;
}

.manage-item.selected {
  background: #f0f4ff;
}

.item-title {
  font-weight: 500;
  font-size: 13px;
  color: #333;
  margin-bottom: 4px;
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.item-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 11px;
  color: #a2a2a2;
}

.badge {
  display: inline-flex;
  align-items: center;
  padding: 1px 7px;
  border-radius: 10px;
  font-size: 10px;
  font-weight: 500;
}

.badge--archive {
  background: #f5f5f5;
  color: #999;
}

.badge--active {
  background: #4F5BDF;
  color: #fff;
}

.badge--important {
  background: #ff9800;
  color: #fff;
}

.status-chip {
  padding: 1px 7px;
  border-radius: 10px;
  font-size: 10px;
  font-weight: 500;
}

.status-chip--active {
  background: #e8f5e9;
  color: #2e7d32;
}

.status-chip--hidden {
  background: #f5f5f5;
  color: #999;
}

.items-footer {
  padding: 10px 16px;
  font-size: 12px;
  color: #a2a2a2;
  border-top: 1px solid #f0f0f0;
  margin-top: auto;
}

/* Правая панель деталей */
.detail-panel {
  flex: 1;
  overflow-y: auto;
  padding: 20px 24px;
}

.detail-panel--empty {
  display: flex;
  align-items: center;
  justify-content: center;
  color: #a2a2a2;
  font-size: 13px;
}

.detail-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 16px;
  gap: 12px;
}

.detail-title {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  color: #1a1a1a;
  flex: 1;
}

.detail-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
  flex-wrap: wrap;
}

.detail-meta {
  display: flex;
  gap: 16px;
  font-size: 12px;
  color: #a2a2a2;
  margin-bottom: 12px;
}

.detail-description {
  font-size: 14px;
  line-height: 1.5;
  color: #333;
  margin: 0 0 16px;
}

.detail-full-text {
  font-size: 14px;
  line-height: 1.6;
  color: #666;
  padding-top: 16px;
  border-top: 1px solid #f0f0f0;
}

/* Анимация переключения вкладок */
.tab-fade-enter-active,
.tab-fade-leave-active {
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.tab-fade-enter-from,
.tab-fade-leave-to {
  opacity: 0;
  transform: translateY(4px);
}

/* Кнопки */
.add-btn {
  min-width: 190px;
}

.lk-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 6px 18px;
  border-radius: 30px;
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  border: 1px solid transparent;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.lk-button--primary {
  background: #4F5BDF;
  color: #fff;
  border-color: #4F5BDF;
}

.lk-button--primary:hover {
  background: #3a45c0;
  border-color: #3a45c0;
}

.lk-button--secondary {
  background: #fff;
  color: #333;
  border-color: #e6e6e6;
}

.lk-button--secondary:hover {
  background: #f5f5f5;
}

.lk-button--danger {
  background: #fff;
  color: #dc2626;
  border-color: #fca5a5;
}

.lk-button--danger:hover {
  background: #ffebee;
}

/* Инпуты */
.lk-input,
.lk-textarea {
  width: 100%;
  padding: 10px 14px;
  border: 1px solid #e6e6e6;
  border-radius: var(--radius-md, 15px);
  font-family: 'Montserrat', sans-serif;
  font-size: 13px;
  box-sizing: border-box;
  transition: border-color 0.2s ease;
  resize: vertical;
}

.lk-input:focus,
.lk-textarea:focus {
  outline: none;
  border-color: #4F5BDF;
}

/* Модалки */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1100;
  backdrop-filter: blur(0.1px);
}

.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: all 0.25s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-from .modal-content,
.modal-fade-leave-to .modal-content {
  opacity: 0;
  transform: scale(0.92) translateY(-16px);
}

.modal-content {
  background: #fff;
  border-radius: 30px;
  width: 560px;
  max-width: 92vw;
  max-height: 82vh;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.25);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 28px 0;
  flex-shrink: 0;
}

.modal-title {
  margin: 0;
  font-size: 17px;
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
  padding: 20px 28px;
  overflow-y: auto;
  flex: 1;
}

.modal-footer {
  padding: 14px 28px 22px;
  border-top: 1px solid #f0f0f0;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  flex-shrink: 0;
}

.form-group {
  margin-bottom: 18px;
}

.form-label {
  display: block;
  font-size: 13px;
  font-weight: 500;
  color: #333;
  margin-bottom: 6px;
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

/* news-body-html от NewsAndReview */
.news-body-html { line-height: 1.6; }
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

/* Список уже card-like (заголовок+бейджи+мета вместо колонок таблицы) -
   rt-table/data-label тут не подходят (нет head-row), карточный вид на узком
   экране собираем локально. Компонент раньше не имел ни одного @media -
   .items-list была фиксирована 360px и сжимала .detail-panel вместо стека. */
@media (max-width: 767.98px) {
  .management-header {
    height: auto;
    padding: 12px 16px;
  }

  .management-body {
    flex-direction: column;
    height: auto;
  }

  .items-list {
    width: 100%;
    max-height: 320px;
    border-right: none;
    border-bottom: 1px solid #e6e6e6;
  }

  .detail-panel {
    width: 100%;
  }

  .manage-item {
    border: 1px solid var(--color-border, #e6e6e6);
    border-radius: var(--radius-md, 15px);
    margin: 0 10px 8px;
  }

  .manage-item:first-child {
    margin-top: 10px;
  }
}
</style>
