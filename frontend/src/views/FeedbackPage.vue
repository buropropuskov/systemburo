<template>
  <section class="feedback-admin">
    <header class="feedback-admin__header">
      <div class="header-row">
        <h2 class="feedback-admin__title">
          Обратная связь
        </h2>
        <RefreshButton
          :loading="loading"
          @refresh="fetchFeedbacks"
        />
      </div>

      <div class="summary-cards">
        <button
          class="summary-card"
          :class="{ 'summary-card--active': statusFilter === null && readFilter === null }"
          @click="resetAllFilters"
        >
          <span class="summary-card__label">Всего</span>
          <span class="summary-card__value">{{ feedbacks.length }}</span>
        </button>
        <button
          class="summary-card summary-card--accent"
          :class="{ 'summary-card--active': readFilter === 'false' }"
          @click="toggleReadFilter('false')"
        >
          <span class="summary-card__label">Новые</span>
          <span class="summary-card__value">{{ unreadCount }}</span>
        </button>
        <button
          class="summary-card summary-card--warning"
          :class="{ 'summary-card--active': statusFilter === 'Не решено' }"
          @click="toggleStatusFilter('Не решено')"
        >
          <span class="summary-card__label">В работе</span>
          <span class="summary-card__value">{{ pendingCount }}</span>
        </button>
        <button
          class="summary-card summary-card--success"
          :class="{ 'summary-card--active': statusFilter === 'Решено' }"
          @click="toggleStatusFilter('Решено')"
        >
          <span class="summary-card__label">Решено</span>
          <span class="summary-card__value">{{ resolvedCount }}</span>
        </button>
      </div>

      <div class="filters-row">
        <SearchComponent
          v-model="searchQuery"
          title="Поиск по имени или сообщению..."
          class="search-component"
          @search="handleSearch"
        />
        <div class="filter-toggle-group">
          <button
            class="lk-button"
            :class="readFilter === 'false' ? 'lk-button--primary' : 'lk-button--ghost'"
            @click="toggleReadFilter('false')"
          >
            Новые
          </button>
          <button
            class="lk-button"
            :class="readFilter === 'true' ? 'lk-button--primary' : 'lk-button--ghost'"
            @click="toggleReadFilter('true')"
          >
            Просмотренные
          </button>
        </div>
      </div>
    </header>

    <div class="feedback-admin__list">
      <SkeletonTransition :loading="loading">
        <template #skeleton>
          <div class="skeleton-grid">
            <SkeletonCard
              v-for="i in 4"
              :key="i"
              :lines="3"
            />
          </div>
        </template>

        <div
          v-if="filteredFeedbacks.length === 0"
          class="empty-state"
        >
          <p>{{ getEmptyMessage() }}</p>
        </div>

        <div
          v-else
          class="cards-grid"
        >
          <article
            v-for="feedback in filteredFeedbacks"
            :key="feedback.id"
            class="ticket-card"
            :class="{
              'ticket-card--unread': !feedback.is_read,
              'ticket-card--resolved': feedback.status === 'Решено'
            }"
          >
            <header class="ticket-card__header">
              <div class="ticket-card__meta">
                <span class="ticket-id">#{{ feedback.id }}</span>
                <span class="ticket-author">{{ feedback.user_name || 'Неизвестный пользователь' }}</span>
              </div>
              <Badge
                :variant="feedback.status === 'Решено' ? 'success' : 'warning'"
                :label="feedback.status"
                size="sm"
              />
            </header>

            <p class="ticket-card__message">
              {{ feedback.message }}
            </p>

            <div
              v-if="feedback.resolution_comment"
              class="ticket-card__response"
            >
              <span class="response-label">Ответ:</span>
              <p>{{ feedback.resolution_comment }}</p>
            </div>

            <footer class="ticket-card__footer">
              <div class="ticket-card__times">
                <span class="ticket-time">{{ formatDateTime(feedback.created_at) }}</span>
                <span
                  v-if="feedback.resolved_at"
                  class="ticket-time ticket-time--resolved"
                >
                  Решено: {{ formatDateTime(feedback.resolved_at) }}
                </span>
              </div>
              <div class="ticket-card__actions">
                <button
                  v-if="!feedback.is_read"
                  class="lk-button lk-button--ghost"
                  :disabled="updating === feedback.id"
                  @click="markAsRead(feedback.id)"
                >
                  Прочитано
                </button>
                <button
                  v-if="feedback.status === 'Не решено'"
                  class="lk-button lk-button--primary"
                  :disabled="updating === feedback.id"
                  @click="openResolveModal(feedback)"
                >
                  Ответить
                </button>
                <button
                  v-if="feedback.status === 'Решено'"
                  class="lk-button lk-button--danger"
                  :disabled="updating === feedback.id"
                  @click="markAsPending(feedback.id)"
                >
                  Вернуть в обращение
                </button>
              </div>
            </footer>
          </article>
        </div>
      </SkeletonTransition>
    </div>

    <Teleport to="body">
      <div
        v-if="resolveTarget"
        class="modal-overlay"
        @click.self="closeResolveModal"
      >
        <div class="modal-content">
          <header class="modal-content__header">
            <h3>Ответ на обращение #{{ resolveTarget.id }}</h3>
            <button
              class="modal-close-btn"
              @click="closeResolveModal"
            >
              ×
            </button>
          </header>
          <section class="modal-content__body">
            <p class="resolve-context"><strong>{{ resolveTarget.user_name || 'Пользователь' }}:</strong> {{ resolveTarget.message }}</p>
            <label class="resolve-label">
              Ответ заявителю
              <textarea
                v-model="resolveCommentValue"
                class="lk-textarea"
                rows="4"
                placeholder="Заявитель увидит ваш ответ. Опишите решение или причину отказа."
              />
            </label>
          </section>
          <footer class="modal-content__footer">
            <button
              class="lk-button lk-button--ghost"
              @click="closeResolveModal"
            >
              Отмена
            </button>
            <button
              class="lk-button lk-button--primary"
              :disabled="updating === resolveTarget.id"
              @click="confirmResolve"
            >
              Отметить решённым
            </button>
          </footer>
        </div>
      </div>
    </Teleport>
  </section>
</template>

<script>
import { apiRequest } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import SearchComponent from '@/components/SearchComponent.vue';
import RefreshButton from '@/components/RefreshButton.vue';
import { SkeletonTransition, SkeletonCard, Badge } from '@/components/ui';

export default {
  name: 'FeedbackPage',
  components: {
    SearchComponent,
    RefreshButton,
    SkeletonTransition,
    SkeletonCard,
    Badge,
  },
  data() {
    return {
      feedbacks: [],
      loading: false,
      updating: null,
      statusFilter: null,
      readFilter: null,
      searchQuery: '',
      searchVariants: [''],
      resolveTarget: null,
      resolveCommentValue: ''
    };
  },
  computed: {
    filteredFeedbacks() {
      let filtered = this.feedbacks;

      if (this.searchQuery.trim() !== '') {
        filtered = filtered.filter(feedback => {
          const userName = (feedback.user_name || '').toLowerCase();
          const message = (feedback.message || '').toLowerCase();
          return this.searchVariants.some(variant => {
            if (!variant) return false;
            return userName.includes(variant) || message.includes(variant);
          });
        });
      }

      if (this.statusFilter) {
        filtered = filtered.filter(f => f.status === this.statusFilter);
      }

      if (this.readFilter !== null) {
        const isRead = this.readFilter === 'true';
        filtered = filtered.filter(f => f.is_read === isRead);
      }

      return filtered.sort((a, b) => {
        if (a.is_read !== b.is_read) return a.is_read ? 1 : -1;
        return new Date(b.created_at) - new Date(a.created_at);
      });
    },
    unreadCount() {
      return this.feedbacks.filter(f => !f.is_read).length;
    },
    pendingCount() {
      return this.feedbacks.filter(f => f.status === 'Не решено').length;
    },
    resolvedCount() {
      return this.feedbacks.filter(f => f.status === 'Решено').length;
    }
  },
  mounted() {
    this.fetchFeedbacks();
  },
  methods: {
    handleSearch(variants) {
      this.searchVariants = variants;
    },
    toggleStatusFilter(status) {
      this.statusFilter = this.statusFilter === status ? null : status;
    },
    toggleReadFilter(state) {
      this.readFilter = this.readFilter === state ? null : state;
    },
    resetAllFilters() {
      this.statusFilter = null;
      this.readFilter = null;
      this.searchQuery = '';
      this.searchVariants = [''];
    },
    async fetchFeedbacks() {
      this.loading = true;
      try {
        const authStore = useAuthStore();
        if (!authStore.token) return;
        const response = await apiRequest('/feedback/all', { method: 'GET' });
        if (response.ok) {
          this.feedbacks = await response.json();
        }
      } catch (e) {
        console.error('Ошибка при загрузке обращений:', e);
      } finally {
        this.loading = false;
      }
    },
    async markAsRead(feedbackId) {
      await this.updateRead(feedbackId, true);
    },
    openResolveModal(feedback) {
      this.resolveTarget = feedback;
      this.resolveCommentValue = '';
    },
    closeResolveModal() {
      this.resolveTarget = null;
      this.resolveCommentValue = '';
    },
    async confirmResolve() {
      const feedbackId = this.resolveTarget.id;
      const comment = this.resolveCommentValue;
      this.updating = feedbackId;
      try {
        const response = await apiRequest(`/feedback/${feedbackId}/status`, {
          method: 'PUT',
          body: JSON.stringify({ status: 'Решено', comment })
        });
        if (response.ok) {
          const idx = this.feedbacks.findIndex(f => f.id === feedbackId);
          if (idx !== -1) {
            this.feedbacks[idx] = {
              ...this.feedbacks[idx],
              status: 'Решено',
              resolution_comment: comment.trim() || null,
              resolved_at: new Date().toISOString(),
              is_read: true
            };
          }
          this.closeResolveModal();
        }
      } catch (e) {
        console.error('Ошибка при выполнении обращения:', e);
      } finally {
        this.updating = null;
      }
    },
    async markAsPending(feedbackId) {
      this.updating = feedbackId;
      try {
        const response = await apiRequest(`/feedback/${feedbackId}/status`, {
          method: 'PUT',
          body: JSON.stringify({ status: 'Не решено' })
        });
        if (response.ok) {
          const idx = this.feedbacks.findIndex(f => f.id === feedbackId);
          if (idx !== -1) {
            this.feedbacks[idx] = {
              ...this.feedbacks[idx],
              status: 'Не решено',
              resolution_comment: null,
              resolved_at: null
            };
          }
        }
      } finally {
        this.updating = null;
      }
    },
    async updateRead(feedbackId, isRead) {
      this.updating = feedbackId;
      try {
        const response = await apiRequest(`/feedback/${feedbackId}/read`, {
          method: 'PUT',
          body: JSON.stringify({ is_read: isRead })
        });
        if (response.ok) {
          const idx = this.feedbacks.findIndex(f => f.id === feedbackId);
          if (idx !== -1) this.feedbacks[idx] = { ...this.feedbacks[idx], is_read: isRead };
        }
      } finally {
        this.updating = null;
      }
    },
    formatDateTime(s) {
      if (!s) return '';
      const d = new Date(s);
      return d.toLocaleString('ru-RU', {
        day: '2-digit', month: '2-digit', year: 'numeric',
        hour: '2-digit', minute: '2-digit'
      });
    },
    getEmptyMessage() {
      if (this.searchQuery.trim() !== '') return 'Нет обращений по вашему запросу';
      if (this.statusFilter !== null || this.readFilter !== null) return 'Нет обращений по выбранным фильтрам';
      return 'Обращений пока нет';
    }
  }
};
</script>

<style scoped>
.feedback-admin {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.feedback-admin__header {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.feedback-admin__title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text);
}

.summary-cards {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.summary-card {
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  cursor: pointer;
  text-align: left;
  transition: border-color 0.15s ease, box-shadow 0.15s ease, transform 0.15s ease;
  font-family: inherit;
}

.summary-card:hover {
  border-color: var(--color-primary);
  box-shadow: var(--shadow-sm);
}

.summary-card--active {
  border-color: var(--color-primary);
  box-shadow: var(--shadow-focus);
}

.summary-card__label {
  font-size: 12px;
  color: var(--color-text-muted);
  font-weight: 500;
}

.summary-card__value {
  font-size: 22px;
  font-weight: 600;
  color: var(--color-text);
}

.summary-card--accent .summary-card__value { color: var(--color-primary); }
.summary-card--warning .summary-card__value { color: #b45309; }
.summary-card--success .summary-card__value { color: #15803d; }

.filters-row {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}

.search-component {
  flex: 1 1 260px;
  max-width: 360px;
}

.filter-toggle-group {
  display: flex;
  gap: 8px;
}

.feedback-admin__list {
  min-height: 240px;
}

.skeleton-grid,
.cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 12px;
}

.ticket-card {
  background: #fff;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.ticket-card:hover {
  border-color: var(--color-primary);
  box-shadow: var(--shadow-sm);
}

.ticket-card--unread {
  border-left: 3px solid var(--color-primary);
}

.ticket-card--resolved {
  background: #fafafa;
}

.ticket-card__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.ticket-card__meta {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.ticket-id {
  font-family: 'JetBrains Mono', monospace, ui-monospace;
  font-size: 12px;
  color: var(--color-text-muted);
  background: var(--color-bg-secondary);
  padding: 2px 8px;
  border-radius: var(--radius-sm);
}

.ticket-author {
  font-weight: 600;
  color: var(--color-text);
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ticket-card__message {
  margin: 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--color-text);
  white-space: pre-wrap;
}

.ticket-card__response {
  background: var(--color-bg);
  border-radius: var(--radius-md);
  padding: 8px 12px;
  border-left: 3px solid var(--color-success);
}

.ticket-card__response p {
  margin: 0;
  font-size: 12px;
  color: var(--color-text);
}

.response-label {
  display: block;
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin-bottom: 4px;
}

.ticket-card__footer {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  flex-wrap: wrap;
  gap: 10px;
  border-top: 1px solid var(--color-border);
  padding-top: 10px;
}

.ticket-card__times {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 11px;
  color: var(--color-text-muted);
}

.ticket-time--resolved {
  color: var(--color-success);
}

.ticket-card__actions {
  display: flex;
  gap: 8px;
}

.empty-state {
  text-align: center;
  padding: 48px 24px;
  color: var(--color-text-muted);
}

.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  z-index: 1000;
}

.modal-content {
  background: #fff;
  border-radius: var(--radius-lg);
  width: 100%;
  max-width: 520px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 16px 40px rgba(15, 23, 42, 0.2);
}

.modal-content__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--color-border);
}

.modal-content__header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.modal-close-btn {
  background: none;
  border: none;
  font-size: 24px;
  cursor: pointer;
  color: var(--color-text-muted);
  line-height: 1;
  padding: 0 4px;
}

.modal-content__body {
  padding: 16px 20px;
  overflow-y: auto;
}

.resolve-context {
  background: var(--color-bg-secondary);
  padding: 10px 12px;
  border-radius: var(--radius-md);
  font-size: 13px;
  margin: 0 0 14px 0;
  line-height: 1.5;
}

.resolve-label {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text);
}

.modal-content__footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 12px 20px;
  border-top: 1px solid var(--color-border);
}

@media (max-width: 768px) {
  .summary-cards {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .filters-row {
    flex-direction: column;
    align-items: stretch;
  }
  .search-component {
    max-width: 100%;
  }
  .filter-toggle-group {
    width: 100%;
  }
  .filter-toggle-group .lk-button {
    flex: 1;
  }
  .cards-grid,
  .skeleton-grid {
    grid-template-columns: 1fr;
  }
}
</style>
