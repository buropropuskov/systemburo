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
        <SearchComponent
          v-model="searchQuery"
          :title="activeTab === 'news' ? 'Поиск новостей...' : 'Поиск объявлений...'"
        />
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
            v-else-if="filteredNewsItems.length === 0"
            class="empty-state"
          >
            {{ newsEmptyText }}
          </div>
          <div
            v-for="item in filteredNewsItems"
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
            Всего: {{ filteredNewsItems.length }}
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
            v-else-if="filteredAnnouncementsItems.length === 0"
            class="empty-state"
          >
            {{ announcementsEmptyText }}
          </div>
          <div
            v-for="item in filteredAnnouncementsItems"
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
            Всего: {{ filteredAnnouncementsItems.length }}
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

    <BaseModal
      :show="showNewsModal"
      :title="editingItem ? 'Редактировать новость' : 'Создать новость'"
      width="640px"
      radius="30px"
      content-testid="news-modal"
      @close="closeNewsModal"
    >
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
      <template #actions>
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
      </template>
    </BaseModal>

    <BaseModal
      :show="showAnnouncementModal"
      :title="editingItem ? 'Редактировать объявление' : 'Создать объявление'"
      width="640px"
      radius="30px"
      content-testid="announcement-modal"
      @close="closeAnnouncementModal"
    >
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
      <template #actions>
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
      </template>
    </BaseModal>
  </div>
</template>

<script>
import { apiRequest } from '@/api/client';
import BaseModal from '@/components/ui/BaseModal.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import TextConstructor from '@/components/TextConstructor.vue';
import SearchComponent from '@/components/SearchComponent.vue';
import { useDeletionsStore } from '@/stores/deletions';
import { useUiStore } from '@/stores/ui';
import { sanitizeHtml } from '@/utils/sanitize.js';
import { buildSearchVariants, matchesSearch } from '@/utils/searchVariants';
import { formatDateTime } from '@/utils/datetime';

export default {
  name: 'NewsManagement',
  components: { BaseModal, RefreshButton, TextConstructor, SearchComponent },
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
      searchQuery: '',
    };
  },
  computed: {
    // Поиск (#1157) - клиентский, по заголовку+описанию, отдельно для
    // каждой вкладки (общий util как в остальных справочниках проекта).
    filteredNewsItems() {
      const variants = buildSearchVariants(this.searchQuery);
      if (!variants.length) return this.newsItems;
      return this.newsItems.filter((item) => matchesSearch(`${item.title} ${item.description || ''}`, variants));
    },
    filteredAnnouncementsItems() {
      const variants = buildSearchVariants(this.searchQuery);
      if (!variants.length) return this.announcementsItems;
      return this.announcementsItems.filter((item) => matchesSearch(`${item.title} ${item.description || ''}`, variants));
    },
    newsEmptyText() {
      return this.searchQuery.trim() ? 'Ничего не найдено по запросу' : 'Нет новостей';
    },
    announcementsEmptyText() {
      return this.searchQuery.trim() ? 'Ничего не найдено по запросу' : 'Нет объявлений';
    },
  },
  watch: {
    // Запрос отфильтровал выбранный элемент из видимого списка - гасим
    // деталь-панель, чтобы она не показывала запись, которой нет в списке.
    searchQuery() {
      if (!this.selectedItem) return;
      const list = this.activeTab === 'news' ? this.filteredNewsItems : this.filteredAnnouncementsItems;
      if (!list.some((item) => item.id === this.selectedItem.id)) {
        this.selectedItem = null;
      }
    },
  },
  mounted() {
    this.fetchAll();
  },
  methods: {
    sanitizeHtml,
    formatDate: formatDateTime,

    switchTab(tab) {
      this.activeTab = tab;
      this.selectedItem = null;
      // Сбрасываем поиск: старый запрос с прошлой вкладки молча отфильтровал
      // бы другой список (поле показывает текст, а список внезапно пуст).
      this.searchQuery = '';
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
  background: var(--surface);
  border: 1px solid var(--border);
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
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.management-title {
  margin: 0;
  font-size: 1.2em;
  font-weight: 600;
  color: var(--text);
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
  background: var(--surface-2);
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
  color: var(--text-muted);
  cursor: pointer;
  border-radius: 17px;
  transition: all 0.2s ease;
}

.tab-btn:hover {
  color: var(--accent-text);
}

.tab-btn.active {
  background: var(--surface);
  color: var(--accent-text);
  box-shadow: 0 1px 3px var(--shadow-drop);
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
  border-right: 1px solid var(--border);
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}

.items-list::-webkit-scrollbar {
  width: 4px;
}

.items-list::-webkit-scrollbar-thumb {
  background: color-mix(in srgb, var(--accent) 22%, var(--surface));
  border-radius: 4px;
}

.empty-state {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
  color: var(--text-muted);
  font-size: 13px;
}

.info-note {
  margin: 10px 12px 0;
  padding: 8px 12px;
  background: var(--accent-tint);
  border-radius: 10px;
  font-size: 12px;
  color: var(--accent-text);
}

.manage-item {
  padding: 14px 16px;
  border-bottom: 1px solid var(--border);
  cursor: pointer;
  transition: background 0.15s ease;
}

.manage-item:hover {
  background: var(--surface-2);
}

.manage-item.selected {
  background: var(--accent-tint);
}

.item-title {
  font-weight: 500;
  font-size: 13px;
  color: var(--text);
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
  color: var(--text-muted);
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
  background: var(--surface-2);
  color: var(--text-muted);
}

.badge--active {
  background: var(--accent);
  color: var(--accent-contrast);
}

.badge--important {
  background: var(--warning);
  color: var(--fill-text);
}

.status-chip {
  padding: 1px 7px;
  border-radius: 10px;
  font-size: 10px;
  font-weight: 500;
}

.status-chip--active {
  background: var(--success-bg);
  color: var(--success-text);
}

.status-chip--hidden {
  background: var(--surface-2);
  color: var(--text-muted);
}

.items-footer {
  padding: 10px 16px;
  font-size: 12px;
  color: var(--text-muted);
  border-top: 1px solid var(--border);
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
  color: var(--text-muted);
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
  color: var(--text);
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
  color: var(--text-muted);
  margin-bottom: 12px;
}

.detail-description {
  font-size: 14px;
  line-height: 1.5;
  color: var(--text);
  margin: 0 0 16px;
}

.detail-full-text {
  font-size: 14px;
  line-height: 1.6;
  color: var(--text-muted);
  padding-top: 16px;
  border-top: 1px solid var(--border);
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
  background: var(--accent);
  color: var(--accent-contrast);
  border-color: var(--accent);
}

.lk-button--primary:hover {
  background: var(--accent-hover);
  border-color: var(--accent-hover);
}

.lk-button--secondary {
  background: var(--surface);
  color: var(--text);
  border-color: var(--border);
}

.lk-button--secondary:hover {
  background: var(--surface-2);
}

.lk-button--danger {
  background: var(--surface);
  color: var(--danger-text);
  border-color: color-mix(in srgb, var(--danger) 30%, var(--surface));
}

.lk-button--danger:hover {
  background: var(--danger-bg);
}

/* Инпуты */
.lk-input,
.lk-textarea {
  width: 100%;
  padding: 10px 14px;
  border: 1px solid var(--border);
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
  border-color: var(--accent);
}

/* Модалки */
.form-group {
  margin-bottom: 18px;
}

.form-label {
  display: block;
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
  margin-bottom: 6px;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  font-size: 13px;
  color: var(--text);
}

.checkbox-label input {
  width: 16px;
  height: 16px;
  cursor: pointer;
  accent-color: var(--accent-text);
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

  /* Направление строки берёт на себя глобальный .rt-header-inline
     (responsive-tables.css, !important); здесь только разрешаем перенос
     контролов (таб-группа+поиск+кнопки) на вторую строку, если не влезают. */
  .header-controls {
    flex-wrap: wrap;
    row-gap: 8px;
  }

  .management-body {
    flex-direction: column;
    height: auto;
  }

  .items-list {
    width: 100%;
    max-height: 320px;
    border-right: none;
    border-bottom: 1px solid var(--border);
  }

  .detail-panel {
    width: 100%;
  }

  .manage-item {
    border: 1px solid var(--color-border, var(--border));
    border-radius: var(--radius-md, 15px);
    margin: 0 10px 8px;
  }

  .manage-item:first-child {
    margin-top: 10px;
  }
}
</style>
