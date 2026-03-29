<template>
  <section class="feedback">
    <header class="feedback__header">
      <div class="header-top">
        <h2 class="feedback__title">Обратная связь</h2>
        <div class="header-stats">
          <div class="stat-item">
            <span class="stat-label">Всего:</span>
            <span class="stat-value">{{ feedbacks.length }}</span>
          </div>
          <div class="stat-item" v-if="unreadCount > 0">
            <span class="stat-label">Новые:</span>
            <span class="stat-value stat-new">{{ unreadCount }}</span>
          </div>
        </div>
        <SearchComponent
          title="Поиск по имени или сообщению..."
          v-model="searchQuery"
          @search="handleSearch"
          class="search-component"
        />
        <button
          class="refresh-btn-header"
          @click="fetchFeedbacks"
          :disabled="loading"
        >
          Обновить
        </button>
      </div>
    </header>

    <div class="feedback__filters">
      <div class="filters-row">
        <div class="filters-group">
          <span class="filters-label">Статус:</span>
          <div class="filter-buttons">
            <button
              class="filter-btn"
              :class="{ active: statusFilter === 'Не решено' }"
              @click="toggleStatusFilter('Не решено')"
            >
              Не решено
            </button>
            <button
              class="filter-btn"
              :class="{ active: statusFilter === 'Решено' }"
              @click="toggleStatusFilter('Решено')"
            >
              Решено
            </button>
          </div>
        </div>

        <div class="filters-group">
          <span class="filters-label">Просмотр:</span>
          <div class="filter-buttons">
            <button
              class="filter-btn"
              :class="{ active: readFilter === 'false' }"
              @click="toggleReadFilter('false')"
            >
              Новые
            </button>
            <button
              class="filter-btn"
              :class="{ active: readFilter === 'true' }"
              @click="toggleReadFilter('true')"
            >
              Просмотренные
            </button>
          </div>
        </div>
      </div>
    </div>

    <div class="feedback__list" :class="{ loading }">
      <div v-if="loading" class="loading-state">
        <div class="spinner"></div>
      </div>

      <div v-else-if="filteredFeedbacks.length === 0" class="empty-state">
        <p class="no-data-message">{{ getEmptyMessage() }}</p>
      </div>

      <div v-else class="feedbacks-container">
        <div
          v-for="feedback in filteredFeedbacks"
          :key="feedback.id"
          class="feedback-item"
          :class="{
            unread: !feedback.is_read,
            'feedback-resolved': feedback.status === 'Решено'
          }"
        >
          <div class="feedback-header">
            <div class="user-info">
              <div class="user-name-container">
                <span class="user-name">{{ feedback.user_name || 'Неизвестный пользователь' }}</span>
                <span class="ticket-number">#{{ feedback.id }}</span>
                <span class="unread-dot" v-if="!feedback.is_read"></span>
              </div>
              <div class="user-meta">
                <span class="timestamp">Создано: {{ formatDateTime(feedback.created_at) }}</span>
                <span v-if="feedback.resolved_at" class="timestamp resolved-time">Выполнено: {{ formatDateTime(feedback.resolved_at) }}</span>
              </div>
            </div>

            <div class="status-indicators">
              <span class="status-badge" :class="getStatusClass(feedback.status)">
                {{ feedback.status }}
              </span>
            </div>
          </div>

          <div class="feedback-content">
            <p class="message">{{ feedback.message }}</p>
            <div v-if="feedback.resolution_comment" class="resolution-comment">
              <strong>Ответ заявителю:</strong> {{ feedback.resolution_comment }}
            </div>
          </div>

          <div class="feedback-actions">
            <div class="action-buttons">
              <button
                v-if="!feedback.is_read"
                @click="markAsRead(feedback.id)"
                class="action-btn mark-read-btn"
                :disabled="updating === feedback.id"
              >
                Прочитано
              </button>
              

              <div v-if="feedback.status === 'Не решено'" class="resolve-section">
                <button
                  @click="markAsResolved(feedback.id)"
                  class="action-btn resolve-btn"
                  :disabled="updating === feedback.id"
                >
                  Выполнить
                </button>
                <textarea
                  v-model="resolveComments[feedback.id]"
                  placeholder="Ответ заявителю (тот, кто подал обращение увидит ваш ответ)"
                  class="resolve-comment-input"
                  rows="2"
                  :disabled="updating === feedback.id"
                ></textarea>
                
              </div>

              <button
                v-if="feedback.status === 'Решено'"
                @click="markAsPending(feedback.id)"
                class="action-btn return-btn"
                :disabled="updating === feedback.id"
              >
                Вернуть в обращение
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script>
import SearchComponent from '@/components/SearchComponent.vue';

export default {
  name: 'FeedbackPage',
  components: {
    SearchComponent
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
      resolveComments: {}
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
        if (a.is_read !== b.is_read) {
          return a.is_read ? 1 : -1;
        }
        return new Date(b.created_at) - new Date(a.created_at);
      });
    },

    unreadCount() {
      return this.feedbacks.filter(f => !f.is_read).length;
    }
  },
  methods: {
    handleSearch(variants) {
      this.searchVariants = variants;
    },

    toggleStatusFilter(status) {
      this.statusFilter = this.statusFilter === status ? null : status;
    },

    toggleReadFilter(readState) {
      this.readFilter = this.readFilter === readState ? null : readState;
    },

    async fetchFeedbacks() {
      this.loading = true;
      try {
        const token = localStorage.getItem('token');
        if (!token) {
          console.error('Пользователь не авторизован');
          return;
        }

        const response = await fetch('http://localhost:8080/feedback/all', {
          method: 'GET',
          headers: {
            Authorization: `Bearer ${token}`,
            'Content-Type': 'application/json'
          }
        });

        if (response.ok) {
          const data = await response.json();
          this.feedbacks = data;
        } else {
          console.error('Ошибка при загрузке обращений:', await response.text());
        }
      } catch (error) {
        console.error('Ошибка сети при загрузке обращений:', error);
      } finally {
        this.loading = false;
      }
    },

    async markAsRead(feedbackId) {
      await this.updateFeedback(feedbackId, { is_read: true });
    },

    async markAsResolved(feedbackId) {
      const comment = this.resolveComments[feedbackId] || '';
      this.updating = feedbackId;
      try {
        const token = localStorage.getItem('token');
        if (!token) return;

        const response = await fetch(`http://localhost:8080/feedback/${feedbackId}/status`, {
          method: 'PUT',
          headers: {
            Authorization: `Bearer ${token}`,
            'Content-Type': 'application/json'
          },
          body: JSON.stringify({ status: 'Решено', comment })
        });

        if (!response.ok) {
          const errorData = await response.json().catch(() => ({}));
          throw new Error(errorData.message || 'Ошибка при обновлении статуса');
        }

        const index = this.feedbacks.findIndex(f => f.id === feedbackId);
        if (index !== -1) {
          this.feedbacks[index] = {
            ...this.feedbacks[index],
            status: 'Решено',
            resolution_comment: comment.trim() || null,
            resolved_at: new Date().toISOString()
          };
        }

        delete this.resolveComments[feedbackId];
        await this.markAsRead(feedbackId);
      } catch (error) {
        console.error('Ошибка при выполнении обращения:', error);
        alert(error.message);
      } finally {
        this.updating = null;
      }
    },

    async markAsPending(feedbackId) {
      await this.updateFeedback(feedbackId, { status: 'Не решено' });
    },

    async updateFeedback(feedbackId, updates) {
      this.updating = feedbackId;
      try {
        const token = localStorage.getItem('token');
        let url, method, body;

        if ('is_read' in updates) {
          url = `http://localhost:8080/feedback/${feedbackId}/read`;
          method = 'PUT';
          body = JSON.stringify({ is_read: updates.is_read });
        } else if ('status' in updates) {
          url = `http://localhost:8080/feedback/${feedbackId}/status`;
          method = 'PUT';
          body = JSON.stringify({ status: updates.status });
        } else {
          throw new Error('Неизвестное обновление');
        }

        const response = await fetch(url, {
          method,
          headers: {
            Authorization: `Bearer ${token}`,
            'Content-Type': 'application/json'
          },
          body
        });

        if (response.ok) {
          const index = this.feedbacks.findIndex(f => f.id === feedbackId);
          if (index !== -1) {
            this.feedbacks[index] = { ...this.feedbacks[index], ...updates };
            if (updates.status === 'Не решено') {
              this.feedbacks[index].resolution_comment = null;
              this.feedbacks[index].resolved_at = null;
            }
          }
        } else {
          const errorText = await response.text();
          console.error('Ошибка при обновлении:', errorText);
          alert('Ошибка при обновлении');
        }
      } catch (error) {
        console.error('Ошибка сети при обновлении:', error);
        alert('Ошибка сети');
      } finally {
        this.updating = null;
      }
    },

    formatDateTime(dateTimeString) {
      if (!dateTimeString) return '';
      const date = new Date(dateTimeString);
      return date.toLocaleString('ru-RU', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
      });
    },

    getStatusClass(status) {
      return {
        'status-pending': status === 'Не решено',
        'status-resolved': status === 'Решено'
      };
    },

    getEmptyMessage() {
      if (this.searchQuery.trim() !== '') {
        return 'Нет обращений по вашему запросу';
      }
      if (this.statusFilter !== null || this.readFilter !== null) {
        return 'Нет обращений по выбранным фильтрам';
      }
      return 'Обращений пока нет';
    }
  },
  mounted() {
    this.fetchFeedbacks();
  }
};
</script>

<style scoped>
.feedback {
  padding: 16px;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}

.feedback__header {
  margin-bottom: 16px;
}

.header-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.feedback__title {
  font-size: 18px;
  font-weight: 600;
  color: #000;
  margin: 0;
  white-space: nowrap;
}

.header-stats {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-left: auto;
  margin-right: 16px;
  flex-shrink: 0;
}

.stat-item {
  display: flex;
  align-items: center;
  gap: 6px;
}

.stat-label {
  font-size: 13px;
  color: #666;
}

.stat-value {
  font-size: 13px;
  font-weight: 600;
  color: #333;
  padding: 2px 8px;
  background: #f1f1f1;
  border-radius: 4px;
}

.stat-new {
  color: #dc3545;
  background: #fff5f5;
}

.search-component {
  width: 260px;
  flex-shrink: 0;
}

.refresh-btn-header {
  padding: 6px 16px;
  border: 1px solid #4F5BDF;
  background: white;
  border-radius: 50px;
  cursor: pointer;
  font-size: 13px;
  transition: all 0.2s;
  height: 36px;
  color: #4F5BDF;
  white-space: nowrap;
  font-weight: 500;
  min-width: 90px;
  flex-shrink: 0;
}

.refresh-btn-header:hover:not(:disabled) {
  background: #4F5BDF;
  color: white;
}

.refresh-btn-header:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  border-color: #e6e6e6;
  color: #999;
}

.feedback__filters {
  padding-bottom: 16px;
  margin-bottom: 16px;
  border-bottom: 1px solid #e6e6e6;
}

.filters-row {
  display: flex;
  align-items: center;
  gap: 24px;
  flex-wrap: wrap;
}

.filters-group {
  display: flex;
  align-items: center;
  gap: 12px;
}

.filters-label {
  font-size: 13px;
  color: #666;
  white-space: nowrap;
}

.filter-buttons {
  display: flex;
  gap: 8px;
}

.filter-btn {
  padding: 6px 16px;
  border: 1px solid #e6e6e6;
  background: white;
  border-radius: 50px;
  cursor: pointer;
  font-size: 13px;
  transition: all 0.2s;
  color: #666;
  white-space: nowrap;
  height: 36px;
  box-sizing: border-box;
  min-width: 110px;
  text-align: center;
  line-height: 1.2;
}

.filter-btn:hover {
  background: #f5f5f5;
  border-color: #d0d0d0;
}

.filter-btn.active {
  background: #4F5BDF;
  color: white;
  font-weight: 500;
  border-color: #4F5BDF;
}

.feedback__list {
  min-height: 200px;
  border: 1px solid #e6e6e6;
  border-radius: 16px;
  background: #fff;
  overflow: hidden;
}

.feedback__list.loading {
  display: flex;
  align-items: center;
  justify-content: center;
}

.feedbacks-container {
  max-height: 600px;
  overflow-y: auto;
}

.feedback-item {
  padding: 16px;
  border-bottom: 1px solid #f0f0f0;
  transition: background-color 0.2s ease;
}

.feedback-item:last-child {
  border-bottom: none;
}

.feedback-item.unread {
  background: #f8f9fa;
}

.feedback-item.feedback-resolved {
  opacity: 0.9;
  background: #f9f9f9;
}

.feedback-item:hover {
  background-color: #f5f5f5;
}

.feedback-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 8px;
}

.user-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.user-name-container {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.user-name {
  font-weight: 600;
  color: #333;
  font-size: 14px;
}

.ticket-number {
  font-size: 12px;
  color: #888;
  background: #f1f1f1;
  padding: 2px 6px;
  border-radius: 4px;
  font-family: monospace;
}

.unread-dot {
  width: 8px;
  height: 8px;
  background: #4F5BDF;
  border-radius: 50%;
  flex-shrink: 0;
  animation: pulse 2s infinite ease-in-out;
}

@keyframes pulse {
  0% { opacity: 0.8; box-shadow: 0 0 0 0 rgba(79, 91, 223, 0.2); }
  50% { opacity: 1; box-shadow: 0 0 0 4px rgba(79, 91, 223, 0.1); }
  100% { opacity: 0.8; box-shadow: 0 0 0 0 rgba(79, 91, 223, 0); }
}

.user-meta {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.timestamp {
  font-size: 12px;
  color: #888;
}

.resolved-time {
  margin-left: 0;
}

.status-indicators {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-badge {
  padding: 4px 12px;
  border-radius: 50px;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
}

.status-pending {
  background: #fff8e1;
  color: #ff8f00;
  border: 1px solid #ffeaa7;
}

.status-resolved {
  background: #e8f5e9;
  color: #2e7d32;
  border: 1px solid #c8e6c9;
}

.feedback-content {
  margin-bottom: 12px;
}

.message {
  color: #333;
  line-height: 1.5;
  font-size: 13px;
  margin: 0;
  white-space: pre-wrap;
}

.resolution-comment {
  margin-top: 8px;
  padding: 8px 12px;
  background: #f9f9f9;
  border-left: 3px solid #28a745;
  border-radius: 4px;
  font-size: 13px;
  color: #555;
}

.resolution-comment strong {
  color: #333;
}

.feedback-actions {
  display: flex;
  justify-content: flex-start;
}

.action-buttons {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: flex-start;
}

.action-btn {
  padding: 6px 16px;
  border: 1px solid #e6e6e6;
  background: white;
  border-radius: 50px;
  cursor: pointer;
  font-size: 12px;
  transition: all 0.2s;
  color: #333;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 100px;
  font-weight: 500;
  box-sizing: border-box;
  line-height: 1;
}

.mark-read-btn {
  border-color: #4F5BDF;
  color: #4F5BDF;
}

.mark-read-btn:hover:not(:disabled) {
  background: #4F5BDF;
  color: white;
}

.resolve-section {
  display: flex;
  gap: 8px;
  min-width: 200px;
}

.resolve-comment-input {
  width: 300px;
  padding: 6px 12px;
  border: 1px solid #e6e6e6;
  border-radius: 12px;
  font-size: 13px;
  line-height: 1.4;
  transition: border 0.2s;
  background: #fff;
  resize: none;
  min-height: 32px;
}

.resolve-comment-input:focus {
  outline: none;
  border-color: #4F5BDF;
  box-shadow: 0 0 0 2px rgba(79, 91, 223, 0.1);
}

.resolve-btn {
  border-color: #28a745;
  color: #28a745;
}

.resolve-btn:hover:not(:disabled) {
  background: #28a745;
  color: white;
}

.return-btn {
  border-color: #dc3545;
  color: #dc3545;
}

.return-btn:hover:not(:disabled) {
  background: #dc3545;
  color: white;
}

.action-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.loading-state {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
}

.empty-state {
  padding: 40px;
}

.no-data-message {
  text-align: center;
  color: #a2a2a2;
  margin: 0;
  font-size: 13px;
}

.spinner {
  width: 32px;
  height: 32px;
  border: 3px solid #e6e6e6;
  border-top-color: #4F5BDF;
  border-radius: 50%;
  animation: spinner-rotate 1s linear infinite;
}

@keyframes spinner-rotate {
  to { transform: rotate(360deg); }
}

.feedbacks-container::-webkit-scrollbar {
  width: 6px;
}

.feedbacks-container::-webkit-scrollbar-track {
  background: transparent;
  border-radius: 3px;
}

.feedbacks-container::-webkit-scrollbar-thumb {
  background: #D9E2FF;
  border-radius: 3px;
}

.feedbacks-container {
  scrollbar-width: thin;
  scrollbar-color: #D9E2FF transparent;
}

@media (max-width: 768px) {
  .feedback {
    padding: 12px;
  }

  .header-top {
    flex-direction: column;
    align-items: stretch;
  }

  .feedback__title {
    align-self: flex-start;
  }

  .header-stats {
    margin-left: 0;
    margin-right: 0;
    justify-content: flex-start;
  }

  .search-component {
    width: 100%;
  }

  .refresh-btn-header {
    width: 100%;
    max-width: none;
  }

  .filters-row {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }

  .filters-group {
    flex-direction: column;
    align-items: flex-start;
    gap: 6px;
  }

  .filter-buttons {
    width: 100%;
  }

  .filter-btn {
    flex: 1;
    min-width: 0;
  }

  .feedback-header {
    flex-direction: column;
    gap: 8px;
  }

  .status-indicators {
    align-self: flex-start;
  }

  .action-buttons {
    flex-direction: column;
    width: 100%;
  }

  .resolve-section {
    flex-direction: column;
    width: 100%;
  }

  .resolve-comment-input {
    width: 100%;
  }

  .action-btn {
    width: 100%;
  }
}
</style>