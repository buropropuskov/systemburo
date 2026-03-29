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
              :class="{ 'active': statusFilter === 'Нерешено' }"
              @click="toggleStatusFilter('Нерешено')"
            >
              Не решено
            </button>
            <button 
              class="filter-btn"
              :class="{ 'active': statusFilter === 'Решено' }"
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
              :class="{ 'active': readFilter === 'false' }"
              @click="toggleReadFilter('false')"
            >
              Новые
            </button>
            <button 
              class="filter-btn"
              :class="{ 'active': readFilter === 'true' }"
              @click="toggleReadFilter('true')"
            >
              Просмотренные
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Активные обращения -->
    <div class="section">
      <div class="section-header">
        <h3 class="section-title">Активные обращения</h3>
        <span class="section-count">{{ pendingCount }}</span>
      </div>
      
      <div class="feedback__list" :class="{ 'loading': loading }">
        <div v-if="loading" class="loading-state">
          <div class="spinner"></div>
        </div>
        
        <div v-else-if="pendingFeedbacks.length === 0" class="empty-state">
          <p class="no-data-message">{{ getEmptyMessage() }}</p>
        </div>

        <div v-else class="feedbacks-container">
          <div 
            v-for="feedback in pendingFeedbacks" 
            :key="feedback.id" 
            class="feedback-item"
            :class="{ 
              'unread': !feedback.is_read,
              'feedback-resolved': feedback.status === 'Решено'
            }"
          >
            <div class="feedback-header">
              <div class="user-info">
                <div class="user-name-container">
                  <span class="user-name">{{ feedback.user_name || 'Неизвестный пользователь' }}</span>
                  <span class="unread-dot" v-if="!feedback.is_read"></span>
                </div>
                <div class="user-meta">
                  <span class="timestamp">{{ formatDateTime(feedback.created_at) }}</span>
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
            </div>

            <div class="feedback-actions">
              <div class="action-buttons">
                <button 
                  v-if="!feedback.is_read"
                  @click="markAsRead(feedback.id)"
                  class="action-btn mark-read-btn"
                  :disabled="updating"
                >
                  Прочитано
                </button>
                
                <button 
                  v-if="feedback.status === 'Нерешено'"
                  @click="markAsResolved(feedback.id)"
                  class="action-btn resolve-btn"
                  :disabled="updating"
                >
                  Выполнить
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Решенные обращения -->
    <div class="section resolved-section">
      <div class="section-header">
        <h3 class="section-title">Решенные обращения</h3>
        <span class="section-count">{{ resolvedCount }}</span>
      </div>
      
      <div class="feedback__list resolved-list">
        <div v-if="resolvedFeedbacks.length === 0" class="empty-state">
          <p class="no-data-message">{{ getEmptyMessage() }}</p>
        </div>

        <div v-else class="feedbacks-container">
          <div 
            v-for="feedback in resolvedFeedbacks" 
            :key="feedback.id" 
            class="feedback-item"
            :class="{ 
              'unread': !feedback.is_read,
              'feedback-resolved': feedback.status === 'Решено'
            }"
          >
            <div class="feedback-header">
              <div class="user-info">
                <div class="user-name-container">
                  <span class="user-name">{{ feedback.user_name || 'Неизвестный пользователь' }}</span>
                  <span class="unread-dot" v-if="!feedback.is_read"></span>
                </div>
                <div class="user-meta">
                  <span class="timestamp">{{ formatDateTime(feedback.created_at) }}</span>
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
            </div>

            <div class="feedback-actions">
              <div class="action-buttons">
                <button 
                  @click="markAsPending(feedback.id)"
                  class="action-btn return-btn"
                  :disabled="updating"
                >
                  Вернуть в обращение
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script>
import { apiRequest } from '@/api/client'
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
      searchVariants: ['']
    };
  },
  computed: {
    filteredFeedbacks() {
      let filtered = this.feedbacks;
      
      // Применяем поиск
      if (this.searchQuery.trim() !== '') {
        filtered = filtered.filter(feedback => {
          const userName = (feedback.user_name || '').toLowerCase();
          const message = (feedback.message || '').toLowerCase();
          
          // Проверяем все варианты поиска
          return this.searchVariants.some(variant => {
            if (!variant) return false;
            return userName.includes(variant) || message.includes(variant);
          });
        });
      }
      
      // Применяем фильтры по статусу
      if (this.statusFilter) {
        filtered = filtered.filter(f => f.status === this.statusFilter);
      }
      
      // Применяем фильтры по прочтению
      if (this.readFilter !== null) {
        const isRead = this.readFilter === 'true';
        filtered = filtered.filter(f => f.is_read === isRead);
      }
      
      // Сначала непрочитанные, потом по дате (новые сверху)
      return filtered.sort((a, b) => {
        if (a.is_read !== b.is_read) {
          return a.is_read ? 1 : -1;
        }
        return new Date(b.created_at) - new Date(a.created_at);
      });
    },
    
    pendingFeedbacks() {
      return this.filteredFeedbacks.filter(f => f.status === 'Нерешено');
    },
    
    resolvedFeedbacks() {
      return this.filteredFeedbacks.filter(f => f.status === 'Решено');
    },
    
    pendingCount() {
      return this.filteredFeedbacks.filter(f => f.status === 'Нерешено').length;
    },
    
    resolvedCount() {
      return this.filteredFeedbacks.filter(f => f.status === 'Решено').length;
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
      if (this.statusFilter === status) {
        this.statusFilter = null;
      } else {
        this.statusFilter = status;
      }
    },
    
    toggleReadFilter(readState) {
      if (this.readFilter === readState) {
        this.readFilter = null;
      } else {
        this.readFilter = readState;
      }
    },
    
    async fetchFeedbacks() {
      this.loading = true;
      try {
        const token = localStorage.getItem("token");
        if (!token) {
          console.error("Пользователь не авторизован");
          return;
        }

        const response = await apiRequest("/feedback/all", {
          method: "GET",
        });

        if (response.ok) {
          const data = await response.json();
          this.feedbacks = data;
        } else {
          console.error("Ошибка при загрузке обращений:", await response.text());
        }
      } catch (error) {
        console.error("Ошибка сети при загрузке обращений:", error);
      } finally {
        this.loading = false;
      }
    },
    
    async markAsRead(feedbackId) {
      await this.updateFeedback(feedbackId, { is_read: true });
    },
    
    async markAsResolved(feedbackId) {
      await this.updateFeedback(feedbackId, { status: 'Решено' });
    },
    
    async markAsPending(feedbackId) {
      await this.updateFeedback(feedbackId, { status: 'Нерешено' });
    },
    
    async updateFeedback(feedbackId, updates) {
      this.updating = feedbackId;
      try {
        const token = localStorage.getItem("token");
        
        let url, method, body;
        
        if ('is_read' in updates) {
          url = `/feedback/${feedbackId}/read`;
          method = 'PUT';
          body = JSON.stringify({ is_read: updates.is_read });
        } else if ('status' in updates) {
          url = `/feedback/${feedbackId}/status`;
          method = 'PUT';
          body = JSON.stringify({ status: updates.status });
        } else {
          throw new Error('Неизвестное обновление');
        }
        
        const response = await apiRequest(url, {
          method,
          body
        });

        if (response.ok) {
          // Обновляем локально
          const index = this.feedbacks.findIndex(f => f.id === feedbackId);
          if (index !== -1) {
            this.feedbacks[index] = { ...this.feedbacks[index], ...updates };
          }
        } else {
          const errorText = await response.text();
          console.error("Ошибка при обновлении:", errorText);
        }
      } catch (error) {
        console.error("Ошибка сети при обновлении:", error);
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
        'status-pending': status === 'Нерешено',
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
}
</script>

<style scoped>
.feedback {
  padding: 12px;
}

.feedback__header {
  padding-bottom: 8px;
  margin-bottom: 8px;
}

.header-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 4px;
  flex-wrap: wrap;
}

.feedback__title {
  font-size: 16px;
  font-weight: 600;
  color: #000;
  margin: 0;
  white-space: nowrap;
}

.header-stats {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-left: auto;
  margin-right: 12px;
  flex-shrink: 0;
}

.stat-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.stat-label {
  font-size: 12px;
  color: #666;
}

.stat-value {
  font-size: 12px;
  font-weight: 600;
  color: #333;
  padding: 1px 6px;
  background: #f1f1f1;
  border-radius: 4px;
}

.stat-new {
  color: #dc3545;
  background: #fff5f5;
}

.search-component {
  width: 220px;
  flex-shrink: 0;
}

.refresh-btn-header {
  padding: 4px 12px;
  border: 1px solid #4F5BDF;
  background: white;
  border-radius: 50px;
  cursor: pointer;
  font-size: 12px;
  transition: all 0.2s;
  height: 28px;
  color: #4F5BDF;
  white-space: nowrap;
  font-weight: 500;
  min-width: 80px;
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
  padding-bottom: 8px;
  margin-bottom: 8px;
  border-bottom: 1px solid #e6e6e6;
}

.filters-row {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.filters-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.filters-label {
  font-size: 12px;
  color: #666;
  white-space: nowrap;
}

.filter-buttons {
  display: flex;
  gap: 6px;
}

.filter-btn {
  padding: 4px 12px;
  border: 1px solid #e6e6e6;
  background: white;
  border-radius: 50px;
  cursor: pointer;
  font-size: 12px;
  transition: all 0.2s;
  color: #666;
  white-space: nowrap;
  height: 28px;
  box-sizing: border-box;
  min-width: 100px;
  text-align: center;
  line-height: 1;
  border-width: 1px;
  position: relative;
  top: 0;
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
  border-width: 1px;
}

.filter-btn:active {
  top: 0;
}

/* Секции */
.section {
  background-color: #fff;
  border-radius: 20px;
  border: 1px solid #e6e6e6;
  overflow: hidden;
  margin-bottom: 12px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: #f8f9fa;
  border-bottom: 1px solid #e6e6e6;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: #333;
  margin: 0;
}

.section-count {
  font-size: 12px;
  font-weight: 500;
  color: #666;
  background: #fff;
  padding: 2px 8px;
  border-radius: 12px;
  min-width: 24px;
  text-align: center;
}

.resolved-section .section-header {

}

.resolved-section .section-title {
  color: #666;
}

/* Стили списков */
.feedback__list {
  height: fit-content;
  display: flex;
  flex-direction: column;
}

.feedback__list.loading {
  height: 100px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.resolved-list {
  max-height: 300px;
  overflow-y: auto;
}

.loading-state {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.empty-state {
  padding: 20px;
}

.no-data-message {
  text-align: center;
  color: #a2a2a2;
  margin: 0;
  font-size: 12px;
}

.feedbacks-container {
  overflow-y: auto;
  flex-grow: 1;
}

.feedback-item {
  padding: 10px;
  border-bottom: 1px solid #f0f0f0;
  transition: background-color 0.2s ease;
  position: relative;
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
  margin-bottom: 6px;
}

.user-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.user-name-container {
  display: flex;
  align-items: center;
  gap: 6px;
}

.user-name {
  font-weight: 600;
  color: #333;
  font-size: 13px;
}

.unread-dot {
  width: 6px;
  height: 6px;
  background: #4F5BDF;
  border-radius: 50%;
  flex-shrink: 0;
  animation: pulse 2.5s infinite ease-in-out;
  position: relative;
}

@keyframes pulse {
  0% {
    opacity: 0.8;
    box-shadow: 0 0 0 0 rgba(79, 91, 223, 0.2);
  }
  50% {
    opacity: 1;
    box-shadow: 0 0 0 3px rgba(79, 91, 223, 0.1);
  }
  100% {
    opacity: 0.8;
    box-shadow: 0 0 0 0 rgba(79, 91, 223, 0);
  }
}

.user-meta {
  display: flex;
  gap: 8px;
  align-items: center;
}

.timestamp {
  font-size: 11px;
  color: #888;
}

.status-indicators {
  display: flex;
  align-items: center;
  gap: 6px;
}

.status-badge {
  padding: 3px 10px;
  border-radius: 50px;
  font-size: 11px;
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
  margin-bottom: 8px;
}

.message {
  color: #333;
  line-height: 1.4;
  font-size: 12px;
  margin: 0;
  white-space: pre-wrap;
}

.feedback-actions {
  display: flex;
  justify-content: flex-start;
}

.action-buttons {
  display: flex;
  gap: 6px;
}

.action-btn {
  padding: 4px 12px;
  border: 1px solid #e6e6e6;
  background: white;
  border-radius: 50px;
  cursor: pointer;
  font-size: 11px;
  transition: all 0.2s;
  color: #333;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 80px;
  font-weight: 500;
  box-sizing: border-box;
  line-height: 1;
  position: relative;
  top: 0;
}

.mark-read-btn {
  border-color: #4F5BDF;
  color: #4F5BDF;
}

.mark-read-btn:hover:not(:disabled) {
  background: #4F5BDF;
  color: white;
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

.action-btn:active {
  top: 0;
}

.spinner {
  width: 24px;
  height: 24px;
  border: 2px solid #e6e6e6;
  border-top-color: #4F5BDF;
  border-radius: 50%;
  animation: spinner-rotate 1s linear infinite;
}

@keyframes spinner-rotate {
  0% {
    transform: rotate(0deg);
  }
  100% {
    transform: rotate(360deg);
  }
}

/* Стили для скроллбара */
.feedbacks-container::-webkit-scrollbar,
.resolved-list::-webkit-scrollbar {
  width: 4px;
}

.feedbacks-container::-webkit-scrollbar-track,
.resolved-list::-webkit-scrollbar-track {
  background: transparent;
  margin: 1px 0;
  border-radius: 2px;
}

.feedbacks-container::-webkit-scrollbar-thumb,
.resolved-list::-webkit-scrollbar-thumb {
  background: #D9E2FF;
  border-radius: 2px;
}

.feedbacks-container,
.resolved-list {
  scrollbar-width: thin;
  scrollbar-color: #D9E2FF transparent;
}

@media (max-width: 768px) {
  .feedback {
    padding: 8px;
  }
  
  .header-top {
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
  }
  
  .feedback__title {
    align-self: flex-start;
  }
  
  .header-stats {
    margin-left: 0;
    margin-right: 0;
    justify-content: center;
    order: 1;
  }
  
  .search-component {
    width: 100%;
    order: 2;
  }
  
  .refresh-btn-header {
    width: 100%;
    max-width: 200px;
    align-self: center;
    order: 3;
  }
  
  .filters-row {
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
  }
  
  .filters-group {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
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
    gap: 6px;
    align-items: flex-start;
  }
  
  .status-indicators {
    align-self: flex-start;
  }
  
  .action-buttons {
    flex-direction: column;
    width: 100%;
  }
  
  .action-btn {
    width: 100%;
  }
}
</style>